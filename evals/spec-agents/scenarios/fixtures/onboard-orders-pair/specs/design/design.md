# Design: Orders API pair

Onboarded from an existing orders service. `orders-api` is vendored unmodified
(`importAsIs`). `orders-api-next` is the modernize sibling that rebuilds the
same HTTP contract through the coding pipeline and, after a human-merged
parity PR, takes over the wiring.
