import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://mwognicki.github.io/pull-secrets-operator',
  base: '/pull-secrets-operator',
  integrations: [
    starlight({
      title: 'Pull Secrets Operator',
      description: 'Kubernetes operator for replicating Docker pull secrets across namespaces.',
      social: {
        github: 'https://github.com/mwognicki/pull-secrets-operator',
        discord: 'https://discord.com/channels/1483122428132589584',
      },
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Installation', slug: 'getting-started/installation' },
            { label: 'Quickstart', slug: 'getting-started/quickstart' },
          ],
        },
        {
          label: 'Concepts',
          items: [
            { label: 'How it works', slug: 'concepts/how-it-works' },
            { label: 'Namespace selection', slug: 'concepts/namespace-selection' },
          ],
        },
        {
          label: 'Guides',
          items: [
            { label: 'Inline credentials', slug: 'guides/inline-credentials' },
            { label: 'Referenced credentials', slug: 'guides/referenced-credentials' },
            { label: 'Namespace overrides', slug: 'guides/namespace-overrides' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'RegistryPullSecret', slug: 'reference/registry-pull-secret' },
            { label: 'PullSecretPolicy', slug: 'reference/pull-secret-policy' },
            { label: 'Status & conditions', slug: 'reference/status-and-conditions' },
          ],
        },
      ],
    }),
  ],
});
