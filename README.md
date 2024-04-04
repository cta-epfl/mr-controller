# mr-ephemeral-controller

This project implement a flux controller interacting directly with the deployed git repository to create ephemeral environements.

This controller relies on an existing folder containing a template for the environement and duplicate it.
It then replaces template string inside it to fit the environment.

This controller is coupled with https://github.com/cta-epfl/mr-image-build to build docker images required for ephemeral environments.
