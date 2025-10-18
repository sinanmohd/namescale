<div align = center>

# Namescale

[![Badge Matrix]](https://matrix.to/#/#chat:sinanmohd.com)

Zeroconf [Wildcard](https://en.wikipedia.org/wiki/Wildcard_DNS_record) [MagicDNS](https://tailscale.com/kb/1081/magicdns) for [Tailnet](https://tailscale.com/kb/1136/tailnet)

</div>

## Deployment

### NixOS

> [!NOTE]
> [Example setup](https://github.com/sinanmohd/nixos/commit/246840e19b230f4cd22b5f40ecf94cc28255b887) on NixOS with ACLs

<details>

<summary>Add namescale to your NixOS flake</summary>

```nix
{
  description = "Bane's NixOS configuration";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";

    namescale = {
      url = "github:sinanmohd/namescale";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = inputs@{ nixpkgs, namescale, ... }: {
    nixosConfigurations = {
      hostname = nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";
        modules = [
          ./configuration.nix
          namescale.nixosModules.namescale
        ];
      };
    };
  };
}
```

</details>

Set up namescale in your `configuration.nix`, here host is the tailscale ip
address assigned to your node running namescale.

```nix
{ ... }: {
    services.namescale = {
        enable = true;
        settings = {
            host = "100.64.0.6";
            port = 53;
            base_domain = "bane.ts.net";
        };
    };
}
```

Using Split DNS make your tailnet to routes all DNS requests to your base domain
to Namescale , on Tailscale you can use the Web GUI for this. if you're using
Headscale you can do the following in your `configuration.nix`.

```nix
{ ... }: {
    services.headscale.settings.dns = {
        base_domain = "bane.ts.net";
        nameservers.split."bane.ts.net" = [ "100.64.0.6" ];
    };
}
```

### GNU/Linux Distros

Make sure Tailscale is up and running on your node and build Namescale

```sh
git clone https://github.com/sinanmohd/namescale.git
cd namescale
go build ./cmd/namescale
```

Run Namescale, here host is the tailscale ip address assigned to your node
running namescale

```sh
./namescale -host "100.64.0.6" -base-domain "bane.ts.net"
```

Using Split DNS make your tailnet to routes all DNS requests to your base domain
to Namescale , on Tailscale you can use the Web GUI for this. if you're using
Headscale you can do the following in your `headscale.yaml`.

```yaml
dns:
  base_domain: bane.ts.net
  nameservers:
    split:
      bane.ts.net:
      - 100.64.0.6
```

### Kubernets & Docker

TODO: i don't use tailnet on docker or k8s, but it should be straight forward
to set up namescale using docker compose or helm charts. if you do, please
open a pr to update the readme

## Development

```sh
# get namescale
git clone https://github.com/sinanmohd/namescale.git
cd namescale

# setup development environment
nix develop

# run checks
nix flake check

# build go binary
go build ./cmd/namescale

# build nix package
nix build

# build and load container image
nix build .#container
docker image load < result
```

<!----------------------------------{ Badges }--------------------------------->
[Badge Matrix]: https://img.shields.io/matrix/chat:sinanmohd.com.svg?label=%23chat%3Asinanmohd.com&logo=matrix&server_fqdn=sinanmohd.com
