# truenas-scale-acme

[![Go Report Card](https://goreportcard.com/badge/github.com/thde/truenas-scale-acme)](https://goreportcard.com/report/github.com/thde/truenas-scale-acme)

`truenas-scale-acme` obtains and manages certificates for TrueNAS Scale using the [ACME DNS-01 challenge](https://letsencrypt.org/docs/challenge-types/#dns-01-challenge) and the [TrueNAS Scale API](https://www.truenas.com/docs/api/scale_rest_api.html).

It uses Caddy's [caddyserver/certmagic](https://github.com/caddyserver/certmagic) library internally to obtain and renew SSL certificates and ensures that TrueNAS uses a valid certificate to serve requests.

## Supported DNS-Providers

Currently the following providers are supported:

- [acme-dns](https://github.com/joohoi/acme-dns)
- [cloudflare](https://github.com/libdns/cloudflare)

If you require a different provider, feel free to create an issue. In theory, all [github.com/libdns](https://github.com/orgs/libdns/repositories?q=&type=all&language=&sort=stargazers) providers can be supported.

## Getting Started

The easiest way is to run `truenas-scale-acme` directly in TrueNAS as an App:

1. [Create](https://www.truenas.com/docs/scale/scaletutorials/toptoolbar/managingapikeys/) an API key in TrueNAS.

1. Register an account on the ACME-DNS server:
   ```shell
   curl -X POST https://auth.acme-dns.io/register
   ```

1. Create a DNS CNAME record pointing from `_acme-challenge.nas.your-domain.com` to the `fulldomain` from the registration response.

1. Create a config directory and ensure the `apps` user has permissions:
   ```shell
   sudo mkdir -p /mnt/tank/acme/certificates
   sudo touch /mnt/tank/acme/config.json
   sudo chmod -R 700 /mnt/tank/acme
   sudo chmod 600 /mnt/tank/acme/config.json
   sudo chown -R apps:apps /mnt/tank/acme/config.json
   ```

1. Update `/mnt/tank/acme/config.json` with the credentials from steps 1 and 2:
   ```json
   {
     "domain": "nas.your-domain.com",
     "scale": {
       "api_key": "your-truenas-api-key",
       "url": "https://172.16.0.1/api/v2.0/",
       "skip_verify": true
     },
     "acme": {
       "email": "acme@example.com",
       "tos_agreed": true,
       "acme-dns": {
         "username": "00000000-0000-0000-0000-000000000000",
         "password": "your-acme-dns-password",
         "subdomain": "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
         "server_url": "https://auth.acme-dns.io"
       }
     }
   }
   ```

1. Add as a custom app in TrueNAS Scale using the following compose, adjusting `source` paths, `user`, and `TZ`:
   ```yaml
   services:
     truenas-scale-acme:
       command:
         - '--daemon'
         - '--schedule'
         - 11 11 * * *
         - '--config'
         - /etc/truenas-scale-acme/config.json
       environment:
         TZ: Europe/Zurich
         XDG_DATA_HOME: /var
       group_add:
         - 568
       healthcheck:
         disable: true
       image: ghcr.io/thde/truenas-scale-acme:latest
       pull_policy: always
       restart: on-failure:10
       user: '3001:3001'
       volumes:
         - type: bind
           source: /mnt/tank/acme/config.json
           target: /etc/truenas-scale-acme/config.json
           read_only: true
         - type: bind
           source: /mnt/tank/acme/certificates
           target: /var/truenas-scale-acme
   ```

## CA's

`truenas-scale-acme` currently has the following CA's configured by default:

1. Let's Encrypt
2. ZeroSSL

This ensures a valid certificate even if one CA is unavailable.

## Other Solutions

- [TrueNAS SCALE/ACME Certificates](https://www.truenas.com/docs/scale/scaletutorials/credentials/certificates/settingupletsencryptcertificates/) - TrueNAS Scale integrated ACME functionality using DNS authentication. Includes support for external [shell commands](https://www.truenas.com/community/threads/howto-acme-dns-authenticator-shell-script-using-acmesh-project.107252/).
- [danb35/deploy-freenas](https://github.com/danb35/deploy-freenas) - Python script to deploy TLS certificates to a TrueNAS Core using its API.
- [acmesh-official/acme.sh/deploy/truenas.sh](https://github.com/acmesh-official/acme.sh/wiki/deployhooks#25-deploy-the-cert-on-truenas-core-server) - acme.sh deploy script for TrueNAS Core using its API.
