# Gravitas deployment

The existing Go server and hosted React client run at `https://news.gravitas.cool`.
The GitHub `Gravitas` workflow tests both container build stages, publishes an
AMD64 image to GHCR, and automatically deploys a digest-pinned release after successful pushes to `main`. Deployment uses the
`gravitas` environment's `GRAVITAS_KUBECONFIG`, scoped to namespace `rss`.

`bootstrap.json` is administrator-only namespace/RBAC setup. `resources.json`
contains the private ClusterIP Service, Deployment, retained 20Gi PVC,
NetworkPolicy and HTTPS Ingress. Run `deploy.sh ghcr.io/jeethsuresh/rss@sha256:...`
with the intended kubeconfig; it rejects any other cluster endpoint and requires
public HTTPS health to pass. No application or cloud credentials belong in Git.

RSS is a fresh installation. Account bootstrap ran privately with registration
bound to loopback, then removed its temporary Secret/Job. The hosted app now directs registration and sign-in to Gravitas IAM; its central
invite and approval policy applies. Local password registration/sign-in is disabled
in this deployment. Session cookies are Secure/HttpOnly, and optional AI credentials
belong in the administrator-managed `rss-runtime` Secret. AI is initially off.

The app uses one replica and Recreate so one process owns the SQLite volume.
Updates have a short interruption. The container has a read-only root,
non-root UID, no Linux capabilities or API token, bounded temporary storage,
startup/readiness/liveness probes, and matching 100m CPU / 256Mi requests and
limits. Network policy permits inbound traffic only from Traefik, and outbound
DNS and public HTTP(S); the existing server also guards against private-address
feed and crawl requests.

## Shared infrastructure

`infrastructure/` is administered here for all Gravitas apps. Traefik chart
42.1.0 exposes one DigitalOcean TCP load balancer, `gravitas-public-ingress`
(209.38.8.79), with two ingress replicas. Applications use ClusterIP services;
DigitalOcean's ingress LoadBalancer uses its normal worker-node transport.

Namecheap keeps the main DNS zone and static apex/wildcard records. Only
`acme.gravitas.cool` is delegated by NS to DigitalOcean DNS. The static CNAME
`_acme-challenge.gravitas.cool` points to
`_acme-challenge.acme.gravitas.cool`. cert-manager 1.21.2 follows that CNAME,
uses the DNS-scoped `cert-manager/digitalocean-dns` Secret, and issues/renews one
ECDSA certificate covering both `gravitas.cool` and `*.gravitas.cool`.
**There is no Namecheap credential or Namecheap DNS solver in the cluster.**

The certificate is `traefik/gravitas-wildcard`, Secret
`traefik/gravitas-wildcard-tls`. TLSStore `traefik/default` supplies the default
certificate. `wildcard-binding.yaml` explicitly binds the same Secret through
an IngressRoute so strict SNI recognizes its SANs. Its reserved
`tls-status.gravitas.cool` route serves only `noop@internal`, never a dashboard.
TLS 1.2 minimum and strict SNI stay enabled; HTTP redirects to HTTPS.

New app Ingresses use class `traefik`, annotations
`traefik.ingress.kubernetes.io/router.entrypoints: websecure` and
`traefik.ingress.kubernetes.io/router.tls: "true"`, and `spec.tls.hosts` with no
secretName. The wildcard DNS record and shared certificate cover any new direct
subdomain; each app still needs its own explicit route, private Service and auth.
No certificate copying or additional load balancer is needed.

Apply the pinned Helm values, `certificates.yaml` and `wildcard-binding.yaml`
with administrator credentials. Provision the DNS-only token Secret separately.
Cluster infrastructure is deliberately outside application CI permissions.

## Operations

Inspect certificate Ready status, expiry and renewal with `kubectl -n traefik
get certificate gravitas-wildcard -o yaml`; inspect Orders/Challenges when DNS
validation fails. cert-manager renews 30 days before expiry and Traefik watches
Secret updates. The DNS token must retain access to the delegated zone.

Deployment credentials are revocable Kubernetes service-account token Secrets
`rss/github-deployer-token` and `office/github-deployer-token`, stored only in
the matching GitHub environment. Rotate them deliberately; they have no built-in
expiry. The Role denies direct Secret reads and any other namespace access.
As with any app deployer, changing a pod can access that app's own mounted data
and runtime credentials. Never use the cluster administrator kubeconfig in CI.

Storage uses `do-block-storage-retain`. A retained volume is persistent storage,
not an offsite backup. For SQLite, use its online backup API, or stop the app
before taking a volume snapshot; do not copy independent live WAL files. Test
restores before relying on a backup. No automatic offsite backup is configured.
For rollback, deploy a previously verified digest; migrations are forward-only,
so do not assume an older binary supports a newer database schema.

The first DNS update preserved all existing record names/types/values, mail
priorities and TTLs. Namecheap's full-record API changed `vpn`'s UI metadata
from dynamic to regular A; its address remains unchanged. The dynamic update
client itself was not exercised during this deployment.

## Gravitas federated login

The hosted deployment uses the custom IAM authorization-code + PKCE S256 protocol
at `https://gravitas.cool`, with only `profile.name` access. `/auth/gravitas/start`
initiates login; `/auth/gravitas/register` starts central registration while
preserving the app callback. An existing IAM session is reused; first-time app
consent may still be required. Central signup retains its invite and approval
rules. This is the IAM server's protocol, not OIDC.

IAM's shared `__Secure-gravitas_signed_in=1` cookie is only a navigation hint.
It is Secure/HttpOnly with Domain `gravitas.cool`, Path `/` and SameSite=Lax;
IAM manages its remaining refresh-family lifetime (at most 30 days). The public
web config exposes only an `autoSignIn` boolean. When no app session exists,
the hosted client attempts `prompt=none` once. A validated `consent_required`
callback starts a fresh interactive consent flow; `login_required` and local
logout return to `auto=off` to prevent loops. Safe local document fragments are
preserved. A forged marker never authenticates an API request. The IAM refresh
credential remains host-only at IAM; apps receive no refresh token.

`GRAVITAS_ISSUER`, `GRAVITAS_APP_URL` and `GRAVITAS_REQUIRED=true` select the hosted
mode. `GRAVITAS_SESSION_KEY` is a base64url 32-byte secret in `rss-runtime` used
to encrypt PKCE transaction and app session cookies. It must persist across rollouts;
no issuer cookie, password, access token or refresh token is shared with the app
browser. Discovery and token/profile requests stay pinned to the configured HTTPS
issuer, and the callback checks encrypted browser state and a one-use PKCE code.

Local password login and registration endpoints cannot authenticate in hosted
mode. Each `(issuer, sub)` receives a separate tenant account; usernames never
link an existing local account. Existing local users/data remain in the database.
The app session uses a host-only Secure/HttpOnly `__Host-rss_session` cookie and
expires after eight hours. Cookie-authenticated writes require the CSRF header.
Local app logout revokes its RSS session; it does not log out of IAM. Each app session is bound to the IAM browser family that created it.

Readiness uses `/auth/gravitas/session-ready.json` to reject a server without the IAM
guard; old images cannot become ready under this manifest. Take an online SQLite
backup or quiesced snapshot before applying the new identity migration. Rollback
must retain the authentication-aware readiness check and an IAM-capable image.

Egress permits the shared Traefik HTTPS backend so IAM discovery and token
exchange continue working after ingress service translation. Other private
application endpoints remain blocked by the network policy.

## Central logout

The token exchange supplies a confidential status-only proof bound to the IAM
browser family, principal and app grant. Only the encrypted host-only app cookie
retains it. Every protected HTTP request checks the pinned, discovered
`session_check_endpoint`, requiring matching issuer, subject, app origin and family
plus a future expiry. There is no positive status cache. Central logout or grant
revocation returns 401 and denies the next request; network failures, non-401
errors and malformed or mismatched 200 responses return 503 without clearing the
app cookie. Legacy cookies without a family proof require a fresh handshake.

Open tabs check their app session every 15 seconds and on focus/resume. A confirmed
revocation clears the authenticated UI and refreshes the navigation hint before
considering passive sign-in. An IAM outage retains local session state. Own app
logout stays local and works during an IAM outage. Independent browser families
remain signed in, and consent grants are retained after ordinary central logout.

Readiness requires this revocation-aware server. Rollback must use a digest that
implements these checks; do not relax the readiness path to an older image.

RSS binds the encrypted proof to the same immutable subject as its database
session. Event streams revalidate periodically and before each event, stopping
with an auth-required/auth-unavailable notification instead of leaking events.

## Private shared deployment tooling

The deployment job checks out `jeethsuresh/gravitas-deploy` at a full commit SHA,
using this repository’s read-only tooling key from the main-only `gravitas`
environment. `.gravitas/deploy.json` selects this app’s existing release script;
its migrations, manifests and public checks remain in this repository. The shared
action checks namespace permissions, exact image digest, readiness and HTTPS,
and removes its temporary kubeconfig on failure or success. Build/test jobs are
unchanged. Review a new tooling pin before updating it. New-app onboarding is
documented in the private tooling repository.
