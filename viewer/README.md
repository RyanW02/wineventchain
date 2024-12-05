# Viewer
## Introduction
This module contains the frontend single page application (SPA) for the web event viewed in the [public](./public)
directory, and an implementation of an HTTP file server to serve the SPA. The frontend application is built using
[Svelte](https://svelte.dev/) and [SvelteKit](https://kit.svelte.dev/), a modern web framework.

## Deployment
It is recommended to deploy the service using the provided [Docker image](../viewer.Dockerfile). The service can be
configured using environment variables in this case.

Alternatively, the service can be deployed outside of a container. Firstly, build the frontend SPA by entering the
`public` directory and running the following command:
```shell
npm install && npm run build
```

The static files will be generated in the `public/build` directory. This path must be given to the file server.

## Configuration
When running outside of a container, a `config.json` file can be used to configure the service. A
[config.json.example](./config.json.example) file has been provided.

Otherwise, the following environment variables can be used to configure the service:
- `PRODUCTION` - Set to `true` to enable production mode. This will change some logging and web server settings.
- `PRETTY_LOGS` - Set to `true` to enable pretty logging. Otherwise, logs will be generated in the JSON format.
- `LOG_LEVEL` - The minimum severity of logs to display. This can be `debug`, `info`, `warn` or `error`.
- `SERVER_ADDRESS` - The address to bind the HTTP server to, e.g. `0.0.0.0:3000`.
- `FRONTEND_BUILD_PATH` - The path to the built frontend SPA, e.g. `./public/build`.
- `FRONTEND_INDEX_FILE` - The name of the frontend SPA's index / fallback file, e.g. `index.html`.