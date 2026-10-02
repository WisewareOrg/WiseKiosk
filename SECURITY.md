# Security policy

## Threat model

WiseKiosk runs on a **single trusted LAN**, one instance per deployment, with no user accounts,
sessions, or authentication. There is no multi-tenancy and no untrusted client. Security comes from
the network boundary, not from the application; one instance serves exactly one configuration and
shares no runtime state with any other
(SYS003<!-- A deployment is parameterised from outside the image -->,
SRS029<!-- One instance, one configuration, nothing shared with another -->).

The controls that are structural, not vigilance-based:

- **API keys resolve server-side only** and never reach the browser by construction — no code path
  carries a secret toward a client, and confinement is never a stripping step
  (SYS003<!-- A deployment is parameterised from outside the image -->).
- **Secrets are delivered by the deployment environment**, never stored in the image, the
  repository, or the configuration
  (SYS003<!-- A deployment is parameterised from outside the image -->).
- **No secret transits through config delivery**, so there is no secret-stripping step to forget —
  the configuration schema offers no secret-bearing key
  (SRS007<!-- Configuration schema offers no secret-bearing key -->).
- **The browser enforces the page's posture**: the page is confined to its own origin and to the
  browser features the display uses
  (SRS010<!-- The display page reaches no origin but the backend's -->,
  SRS027<!-- The display page holds no device capability it does not use -->).
- **Every response carries that posture as a header.** A content-security policy restricts script,
  style, image, font, connection and form-target sources to the page's own origin, forbids embedding
  an object, and confines the document's base URL to that origin too; `X-Content-Type-Options:
  nosniff` backs the concrete type every response declares
  (SRS028<!-- Served responses declare their type, and forbid the browser inferring one -->); and a
  Permissions-Policy denies every browser feature against a committed allowlist, empty until a module
  needs one. `frame-ancestors` and `Referrer-Policy` are deliberately not served: both defend a
  browsing user against another origin, and a display alone on a trusted LAN, embedded by nobody,
  has neither —
  SRS010<!-- The display page reaches no origin but the backend's -->'s rationale records the
  reasoning and its reopening premise. The policy's one
  admission, `style-src-attr 'unsafe-inline'`, is accepted risk: several elements position themselves
  with a runtime-computed inline `style` attribute, which no per-value hash or nonce can cover, and
  the alternative — admitting `'unsafe-inline'` on `style-src` itself — would additionally admit an
  inline `<style>` block this project does not ship.

## Reporting a vulnerability

Report privately via GitHub's **[Private vulnerability reporting](https://github.com/WisewareOrg/WiseKiosk/security/advisories/new)**
(Security → Advisories → Report a vulnerability). Please do not open a public issue for a security
report. A fix or triage response is aimed for within a week.
