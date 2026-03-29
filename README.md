# truenas-scale-acme

[![Go Report Card](https://goreportcard.com/badge/github.com/thde/truenas-scale-acme)](https://goreportcard.com/report/github.com/thde/truenas-scale-acme)

`truenas-scale-acme` obtains and manages certificates for TrueNAS Scale using the [ACME DNS-01 challenge](https://letsencrypt.org/docs/challenge-types/#dns-01-challenge) and the [TrueNAS WebSocket API](https://api.truenas.com).

It uses Caddy's [caddyserver/certmagic](https://github.com/caddyserver/certmagic) library internally to obtain and renew SSL certificates and ensures that TrueNAS uses a valid certificate to serve requests.

## Supported DNS-Providers

Currently the following providers are supported:

- [acme-dns](https://github.com/joohoi/acme-dns)
- [cloudflare](https://github.com/libdns/cloudflare)

If you require a different provider, feel free to create an issue. In theory, all [github.com/libdns](https://github.com/orgs/libdns/repositories?q=&type=all&language=&sort=stargazers) providers can be supported.

## Install

The recommended way to run `truenas-scale-acme` is as a custom application inside TrueNAS:

### TrueNAS Custom App

Deploy via **Apps → Custom App** using the following compose configuration. Adjust the volume paths, schedule, user/group, and timezone to match your setup.

```yaml
services:
  truenas-scale-acme:
    image: ghcr.io/thde/truenas-scale-acme:latest
    command:
      - '--daemon'
      - '--schedule'
      - '11 11 * * *'
      - '--config'
      - /etc/truenas-scale-acme/config.json
    environment:
      TZ: Europe/Zurich
      XDG_DATA_HOME: /var
    user: '3001:3001'
    group_add:
      - 568  # apps group
    restart: on-failure:10
    pull_policy: always
    volumes:
      - type: bind
        source: /mnt/flash/home/acme/config.json
        target: /etc/truenas-scale-acme/config.json
        read_only: true
      - type: bind
        source: /mnt/flash/home/acme/certificates
        target: /var/truenas-scale-acme
```

## Getting Started

1. [Create an API key](https://www.truenas.com/docs/scale/scaletutorials/toptoolbar/managingapikeys/#adding-an-api-key) in TrueNAS.
1. Register an account on an ACME-DNS server:
   ```shell
   curl -X POST https://auth.acme-dns.io/register
   ```
1. Create a DNS CNAME record pointing `_acme-challenge.your-domain.example.com` to the `fulldomain` from the registration response.
1. Create the config file at the path referenced in your compose volume (e.g. `/mnt/flash/home/acme/config.json`):
   ```json
   {
     "domain": "nas.domain.com",
     "api": {
       "api_key": "s3cure",
       "url": "wss://172.16.0.1/api/current",
       "skip_verify": true
     },
     "acme": {
       "email": "myemail@example.com",
       "tos_agreed": true,
       "acme-dns": {
         "username": "00000000-0000-0000-0000-000000000000",
         "password": "s3cure",
         "subdomain": "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
         "server_url": "https://auth.acme-dns.io"
       }
     }
   }
   ```
1. Deploy the custom app and verify in the container logs that the certificate is issued and applied successfully.

## CA's

`truenas-scale-acme` currently has the following CA's configured by default:

1. Let's Encrypt
2. ZeroSSL

This ensures a valid certificate even if one CA is unavailable.

## Other Solutions

- [TrueNAS SCALE/ACME Certificates](https://www.truenas.com/docs/scale/scaletutorials/credentials/certificates/settingupletsencryptcertificates/) - TrueNAS Scale integrated ACME functionality using DNS authentication. Includes support for external [shell commands](https://www.truenas.com/community/threads/howto-acme-dns-authenticator-shell-script-using-acmesh-project.107252/).
- [danb35/deploy-freenas](https://github.com/danb35/deploy-freenas) - Python script to deploy TLS certificates to a TrueNAS Core using its API.
- [acmesh-official/acme.sh/deploy/truenas.sh](https://github.com/acmesh-official/acme.sh/wiki/deployhooks#25-deploy-the-cert-on-truenas-core-server) - acme.sh deploy script for TrueNAS Core using its API.
