# Requirements: Orders API

## Overview

An existing orders HTTP API is being brought onto AEP. The live service
accepts create/get/list order requests. Onboarding vendors it unmodified as
the parity reference and rebuilds a modernize sibling against the same
contract.

## Users & Authentication

- Callers are other services on the intranet. No end-user SSO in v1.

## Functional Requirements

1. Create an order with a customer id and a list of line items.
2. Get an order by id.
3. List orders for a customer.
4. The modernize sibling must match the legacy sibling's responses for the
   same requests (parity).

## Out of scope

- Payments, inventory, or a storefront.
