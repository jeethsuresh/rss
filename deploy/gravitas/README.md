# Gravitas deployment

The existing Go server and hosted React client run at `https://news.gravitas.cool`.
The GitHub `Gravitas` workflow tests both container build stages, publishes an
AMD64 image to GHCR, and deploys a digest-pinned release from `main` when the
repository variable `GRAVITAS_DEPLOY_ENABLED=true`. Deployment uses the
`gravitas` environment's `GRAVITAS_KUBECONFIG`, scoped to namespace `rss`.

`bootstrap.json` is administrator-only namespace/RBAC setup. `resources.json`
contains the private ClusterIP Service, Deployment, retained 20Gi PVC,
NetworkPolicy and HTTPS Ingress. Run `deploy.sh ghcr.io/jeethsuresh/rss@sha256:...`
with the intended kubeconfig; it rejects any other cluster endpoint and requires
public HTTPS health to pass. No application or cloud credentials belong in Git.

RSS is a fresh installation. Account bootstrap ran privately with registration
bound to loopback, then removed its temporary Secret/Job. Public registration
is disabled, session cookies are Secure/HttpOnly, and optional AI credentials
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
