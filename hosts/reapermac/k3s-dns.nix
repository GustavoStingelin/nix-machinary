{ ... }:

{
  # Every k3s ingress answers on the same Traefik LoadBalancer address, and
  # /etc/hosts has no wildcards — dnsmasq's address=/k3s.lan/IP covers the apex
  # and every subdomain, so new ingresses need no config change.
  services.dnsmasq = {
    enable = true;
    bind = "127.0.0.1";
    addresses."k3s.lan" = "192.168.18.220";
  };
}
