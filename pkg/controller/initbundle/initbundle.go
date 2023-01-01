/*
Copyright 2022 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package initbundle

import (
	"context"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	v1 "github.com/stackrox/rox/generated/api/v1"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/stehessel/provider-stackrox/apis/initbundle/v1alpha1"
	apisv1alpha1 "github.com/stehessel/provider-stackrox/apis/v1alpha1"
	"github.com/stehessel/provider-stackrox/pkg/clients/central"
	"github.com/stehessel/provider-stackrox/pkg/features"
)

const (
	errNotInitBundle = "managed resource is not a InitBundle custom resource"
	errTrackPCUsage  = "cannot track ProviderConfig usage"
	errGetPC         = "cannot get ProviderConfig"
	errGetCreds      = "cannot get credentials"
	errGetFailed     = "cannot get init bundle"
	errObserveFailed = "cannot observe init bundle"
	errCreateFailed  = "cannot create init bundle"
	errUpdateFailed  = "cannot update init bundle"
	errDeleteFailed  = "cannot delete init bundle"
)

// Setup adds a controller that reconciles InitBundle managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.InitBundleGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.InitBundleGroupVersionKind),
		managed.WithExternalConnectDisconnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.InitBundle{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube     client.Client
	usage    resource.Tracker
	external *external
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.InitBundle)
	if !ok {
		return nil, errors.New(errNotInitBundle)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	cd := pc.Spec.Credentials
	token, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}
	stringToken := string(token)

	client, err := central.NewGRPC(ctx, pc.Spec.Endpoint, stringToken)
	if err != nil {
		return nil, errors.Wrap(err, central.ErrNewClient)
	}
	c.external = &external{client: client}
	return c.external, nil
}

// Disconnect closes the connection of the external client.
func (c *connector) Disconnect(ctx context.Context) error {
	err := c.external.close()
	return errors.Wrap(err, central.ErrCloseClient)
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	client *grpc.ClientConn
}

func (c *external) close() error {
	if c != nil && c.client != nil {
		err := c.client.Close()
		return errors.Wrap(err, central.ErrCloseClient)
	}
	return nil
}

func generateObservation(in *v1.InitBundleMeta) v1alpha1.InitBundleObservation {
	att := v1alpha1.Attributes{}
	for _, it := range in.CreatedBy.GetAttributes() {
		att[it.GetKey()] = it.GetValue()
	}

	ic := []v1alpha1.ImpactedCluster{}
	for _, it := range in.GetImpactedClusters() {
		ic = append(ic, v1alpha1.ImpactedCluster{ID: it.GetId(), Name: it.GetName()})
	}

	return v1alpha1.InitBundleObservation{
		CreatedAt: metav1.NewTime(time.Unix(in.CreatedAt.GetSeconds(), int64(in.CreatedAt.GetNanos()))),
		CreatedBy: v1alpha1.User{
			Attributes:     att,
			AuthProviderID: in.CreatedBy.GetAuthProviderId(),
			ID:             in.CreatedBy.GetId(),
		},
		ExpiresAt:        metav1.NewTime(time.Unix(in.ExpiresAt.GetSeconds(), int64(in.ExpiresAt.GetNanos()))),
		ID:               in.Id,
		ImpactedClusters: ic,
		Name:             in.Name,
	}
}

func isUpToDate(in *v1alpha1.InitBundle, observed *v1.InitBundleMeta) (bool, string) {
	observedParams := v1alpha1.InitBundleParameters{Name: observed.GetName()}
	if diff := cmp.Diff(in.Spec.ForProvider, observedParams, cmpopts.EquateEmpty()); diff != "" {
		diff = "Observed difference in init bundle\n" + diff
		return false, diff
	}
	return true, ""
}

func (c *external) getInitBundle(ctx context.Context, cr *v1alpha1.InitBundle) (*v1.InitBundleMeta, error) {
	svc := v1.NewClusterInitServiceClient(c.client)
	resp, err := svc.GetInitBundles(ctx, &v1.Empty{})
	if err != nil {
		return nil, errors.Wrap(err, errObserveFailed)
	}
	for _, it := range resp.Items {
		if it.GetName() == meta.GetExternalName(cr) {
			return it, nil
		}
	}
	return nil, nil
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.InitBundle)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotInitBundle)
	}

	bundle, err := c.getInitBundle(ctx, cr)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errObserveFailed)
	}
	if bundle == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider = generateObservation(bundle)
	cr.SetConditions(xpv1.Available())
	upToDate, diff := isUpToDate(cr, bundle)

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
		Diff:             diff,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.InitBundle)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotInitBundle)
	}
	cr.SetConditions(xpv1.Creating())

	svc := v1.NewClusterInitServiceClient(c.client)
	req := v1.InitBundleGenRequest{Name: cr.Spec.ForProvider.Name}
	resp, err := svc.GenerateInitBundle(ctx, &req)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateFailed)
	}

	if m := resp.GetMeta(); m != nil {
		cr.Status.AtProvider = generateObservation(m)
		meta.SetExternalName(cr, m.GetName())
	}
	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{
			"helmValuesBundle": resp.GetHelmValuesBundle(),
			"kubectlBundle":    resp.GetKubectlBundle(),
		},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.InitBundle)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotInitBundle)
	}
	if cr.GetCondition(xpv1.TypeReady) == xpv1.Creating() ||
		cr.GetCondition(xpv1.TypeReady) == xpv1.Deleting() {
		return managed.ExternalUpdate{}, nil
	}

	err := c.Delete(ctx, mg)
	return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateFailed)
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.InitBundle)
	if !ok {
		return errors.New(errNotInitBundle)
	}
	mg.SetConditions(xpv1.Deleting())

	svc := v1.NewClusterInitServiceClient(c.client)
	req := v1.InitBundleRevokeRequest{Ids: []string{cr.Status.AtProvider.ID}}
	_, err := svc.RevokeInitBundle(ctx, &req)
	return errors.Wrap(err, errDeleteFailed)
}
