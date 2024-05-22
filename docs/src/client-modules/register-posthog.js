export function onRouteUpdate() {
  const source = "docs";
  return window.posthog.register_once({ chainloop_source: source });
}
