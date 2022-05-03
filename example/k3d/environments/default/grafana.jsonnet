local grafana = import 'grafana/grafana.libsonnet';
local k = import 'ksonnet-util/kausal.libsonnet';

{
  _images+:: {
    grafana: 'grafana/grafana-oss:8.5.1',
  },

  _config+:: {
    namespace: 'default',
    grafana+: {
      cortex_datasource: 'http://cortex.default.svc.cluster.local:9009/api/prom',
    },
  },

  namespace: k.core.v1.namespace.new($._config.namespace),

  grafana:
    grafana
    + grafana.withImage($._images.grafana)
    + grafana.withAnonymous()
    + grafana.withTheme('dark')
    + grafana.addDatasource(
      'Cortex', grafana.datasource.new(
        'Cortex', $._config.grafana.cortex_datasource, type='prometheus', default=true
      ) { uid: 'cortex' },
    ),

  grafana_ingress:
    local ingress = k.networking.v1.ingress;
    local path = k.networking.v1.httpIngressPath;
    local rule = k.networking.v1.ingressRule;
    ingress.new('grafana-ingress') +
    ingress.mixin.spec.withRules([
      rule.withHost('grafana.k3d.localhost') +
      rule.http.withPaths([
        path.withPath('/')
        + path.withPathType('Prefix')
        + path.backend.service.withName('grafana')
        + path.backend.service.port.withNumber($.grafana._config.port),
      ]),
    ]),
}
