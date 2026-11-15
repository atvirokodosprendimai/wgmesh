package node

// EnforceRouteBinding switches the route drop of R5: a route entry equal to or
// inside the mesh network, or equal to another member's mesh IP, is removed from
// the candidate. It is false until release N+2 (ADR-0002 decision 4); until then
// each such entry is a warning.
var EnforceRouteBinding = false
