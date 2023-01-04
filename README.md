# provider-stackrox

## Work-in-progress

At this point `provider-stackrox` is only a proof of concept and not recommended for productive use.

## Overview

`provider-stackrox` is the Crossplane infrastructure provider for the [Stackrox](https://stackrox.io)
Kubernetes security platform. The provider that is built from the source
code in this repository can be installed into a Crossplane control plane and
adds the following new functionality:

- Custom Resource Definitions (CRDs) and associated controllers to provision and configure
  the Stackrox Central service.

## Getting Started and Documentation

For getting started guides, installation, deployment, and administration, see
our [Documentation](https://crossplane.io/docs).

## Contributing

`provider-stackrox` is a community driven project and we welcome contributions. See the
Crossplane [Contributing](https://github.com/crossplane/crossplane/blob/master/CONTRIBUTING.md)
guidelines to get started.

## Report a Bug

For filing bugs, suggesting improvements, or requesting new features, please
open an [issue](https://github.com/stehessel/provider-stackrox/issues).

## Roadmap

Add support for all of Central's API.

## Code of Conduct

`provider-stackrox` adheres to the same [Code of
Conduct](https://github.com/crossplane/crossplane/blob/master/CODE_OF_CONDUCT.md)
as the core Crossplane project.

## Licensing

`provider-stackrox` is under the Apache 2.0 license.
