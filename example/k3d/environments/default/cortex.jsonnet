local cortex = import 'cortex/main.libsonnet';
local k = import 'ksonnet-util/kausal.libsonnet';

{
  cortex: cortex.new('default'),
  cortex_ingress:
    local ingress = k.networking.v1.ingress;
    local path = k.networking.v1.httpIngressPath;
    local rule = k.networking.v1.ingressRule;
    ingress.new('cortex-ingress') +
    ingress.mixin.spec.withRules([
      rule.withHost('cortex.k3d.localhost') +
      rule.http.withPaths([
        path.withPath('/')
        + path.withPathType('Prefix')
        + path.backend.service.withName('cortex')
        + path.backend.service.port.withNumber($.cortex._config.server.http_listen_port),
      ]),
    ]),
}
