# API explorer

A browsable rendering of [`boundary/openapi.yaml`](https://github.com/WisewareOrg/WiseKiosk/blob/main/boundary/openapi.yaml)
([ADR 0008 rev 6](decisions/0008-boundary-contract-openapi-codegen.md)), the whole wire contract —
routes, request and response shapes — built from [Swagger UI](https://github.com/swagger-api/swagger-ui)
([ADR 0029 rev 2](decisions/0029-api-explorer-swagger-ui.md)). Swagger UI is fetched at build time as
a devDependency and copied into this site; its source is never committed.

On this published site, "Try it out" is reference only — there is no backend behind it. Locally, with
`just serve` running, `just docs-serve` builds and serves this whole docs site at
`http://localhost:5174/`, via a self-contained dev server in `docs/site/` that proxies `/api`,
`/healthz` to the backend — this page is interactive there.

```{raw} html
<link rel="stylesheet" href="_static/swagger-ui/swagger-ui.css">
<div id="swagger-ui"></div>
<script src="_static/swagger-ui/swagger-ui-bundle.js"></script>
<script src="_static/swagger-ui/swagger-ui-standalone-preset.js"></script>
<script>
  window.addEventListener("DOMContentLoaded", function () {
    window.ui = SwaggerUIBundle({
      url: "_static/openapi.yaml",
      dom_id: "#swagger-ui",
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
      layout: "StandaloneLayout",
    });
  });
</script>
```
