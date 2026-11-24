# 0001: Modular monolith

## Context
ATLAS needs strong domain boundaries without distributed-system overhead.

## Decision
Use a Go modular monolith with constructor injection and HTTP as an outer adapter.

## Alternatives
Microservices; global layered folders.

## Consequences
Features remain independently testable and collectors can be added without direct database access.
