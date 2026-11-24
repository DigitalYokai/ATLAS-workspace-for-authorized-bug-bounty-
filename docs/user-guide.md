# ATLAS user guide

1. Copy `.env.example` to `.env`, choose a long administrator password, then start ATLAS with Docker Compose.
2. Sign in with the bootstrap username and password.
3. Create a target and explicitly define included and excluded scope.
4. Start a permitted collection run. When it completes, inspect **Assets** or open the **Command center**.
5. In **Knowledge graph**, enter an asset ID as the root and explore one or two bounded relationship hops.
6. Use **Passive monitoring** to schedule DNS and certificate-transparency checks. Disable schedules or globally pause them in Settings when needed.
7. Export a target as JSON, CSV, Markdown, or graph JSON from the corresponding export endpoint or client integration.

ATLAS stores intelligence observations, not confirmed vulnerabilities. Manual hypotheses remain visibly distinct from automated observations.
