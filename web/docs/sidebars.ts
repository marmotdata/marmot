import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

// The sidebar follows a new reader's journey (see ~/Code/marmot-docs-style/IA.md), not the folder layout, so it is listed by hand and a new page has to be added here to appear. Doc IDs and URLs still come from the file paths.

const doc = (id: string, label?: string) => (label ? {type: 'doc' as const, id, label} : {type: 'doc' as const, id});

const sidebars: SidebarsConfig = {
  tutorialSidebar: [
    doc('introduction', 'Introduction'),
    {
      type: 'category',
      label: 'Get started',
      collapsed: false,
      items: [
        doc('quick-start', 'Run it yourself'),
        // A ref points at the Cloud getting-started page without moving it out of the Cloud section.
        {type: 'ref', id: 'Cloud/getting-started', label: 'Use Marmot Cloud'},
      ],
    },
    {
      type: 'category',
      label: 'Marmot Cloud',
      link: {type: 'doc', id: 'Cloud/index'},
      items: [
        doc('Cloud/getting-started'),
        doc('Cloud/instances'),
        doc('Cloud/members'),
        doc('Cloud/custom-domains'),
        doc('Cloud/access-control'),
        doc('Cloud/grant-access'),
        doc('Cloud/query-access'),
        doc('Cloud/service-accounts'),
        doc('Cloud/single-sign-on'),
        doc('Cloud/pipelines'),
        {
          type: 'category',
          label: 'Connect your cloud',
          items: [doc('Cloud/Guides/google', 'Google Cloud'), doc('Cloud/Guides/aws', 'AWS'), doc('Cloud/Guides/azure', 'Azure')],
        },
        doc('Cloud/secret-stores'),
        doc('Cloud/plugin-registry'),
        doc('Cloud/security'),
      ],
    },
    {
      type: 'category',
      label: 'Self-host',
      link: {type: 'doc', id: 'Deploy/index'},
      items: [
        {
          type: 'category',
          label: 'Install',
          collapsed: false,
          items: [
            doc('Deploy/Docker-Compose', 'Docker Compose'),
            doc('Deploy/Docker', 'Docker'),
            doc('Deploy/Helm', 'Helm and Kubernetes'),
            doc('Deploy/CLI', 'Binary'),
          ],
        },
        doc('Deploy/upgrade-0.13', 'Upgrade to 0.13'),
        {
          type: 'category',
          label: 'Configure',
          items: [
            doc('Configure/index', 'Configuration reference'),
            doc('Configure/tls', 'TLS'),
            doc('Configure/elasticsearch', 'Search backend'),
            doc('Configure/banner', 'Banner'),
            doc('Configure/plugin-updates', 'Plugin updates'),
            doc('metrics', 'Monitoring and metrics'),
            doc('Configure/telemetry', 'Telemetry'),
          ],
        },
      ],
    },
    {
      type: 'category',
      label: 'Populate the catalog',
      link: {type: 'doc', id: 'Populating/index'},
      items: [
        doc('Populating/Pipelines', 'Pipelines'),
        doc('Populating/UI', 'From the UI'),
        doc('Populating/CLI', 'From the CLI'),
        doc('Populating/Terraform', 'Terraform'),
        doc('Populating/Pulumi', 'Pulumi'),
        doc('Populating/Operator', 'Kubernetes operator'),
        doc('open-lineage', 'OpenLineage'),
        doc('Populating/API', 'API'),
        doc('Configure/serving-plugins', 'Serve queries from your network'),
        doc('Plugins/index', 'Plugins'),
        {
          type: 'category',
          label: 'MRNs',
          link: {type: 'doc', id: 'MRNs/index'},
          items: [doc('MRNs/zones', 'Zones')],
        },
      ],
    },
    {
      type: 'category',
      label: 'Use the catalog',
      items: [
        doc('queries', 'Search and the query language'),
        doc('lineage', 'Lineage'),
        {
          type: 'category',
          label: 'Query data',
          link: {type: 'doc', id: 'data-queries'},
          items: [doc('query-data', 'Query your data'), doc('query-audit', 'Query audit')],
        },
        doc('glossary', 'Glossary'),
        doc('data-products', 'Data products'),
        doc('asset-rules', 'Asset rules'),
        {
          type: 'category',
          label: 'Notifications',
          link: {type: 'doc', id: 'Notifications/index'},
          items: [doc('Notifications/webhooks', 'Webhooks')],
        },
      ],
    },
    {
      type: 'category',
      label: 'Agents and MCP',
      link: {type: 'doc', id: 'Agents/index'},
      items: [
        doc('MCP/index', 'MCP server'),
        doc('MCP/skills', 'Skills'),
        doc('Agents/langchain', 'LangChain'),
        doc('Agents/claude-agent', 'Claude Agent SDK'),
      ],
    },
    {
      type: 'category',
      label: 'Users and access',
      items: [
        doc('Configure/access-control', 'Access control'),
        {
          type: 'category',
          label: 'Single sign-on',
          link: {type: 'doc', id: 'Configure/Authentication/index'},
          items: [
            doc('Configure/Authentication/auth0-oidc', 'Auth0'),
            doc('Configure/Authentication/generic-oidc', 'Generic OIDC'),
            doc('Configure/Authentication/github-oauth', 'GitHub'),
            doc('Configure/Authentication/gitlab-oidc', 'GitLab'),
            doc('Configure/Authentication/google-oidc', 'Google'),
            doc('Configure/Authentication/keycloak-oidc', 'Keycloak'),
            doc('Configure/Authentication/okta-oidc', 'Okta'),
            doc('Configure/Authentication/slack-oidc', 'Slack'),
          ],
        },
        doc('Configure/anonymous-access', 'Anonymous access'),
      ],
    },
    {
      type: 'category',
      label: 'Build on Marmot',
      items: [doc('SDK/index', 'SDK'), doc('Develop/creating-plugins', 'Build a plugin')],
    },
    {
      type: 'category',
      label: 'Contribute',
      items: [doc('Develop/local-development', 'Local development'), doc('Develop/translations', 'Translations')],
    },
    {
      type: 'category',
      label: 'Reference',
      items: [doc('cli', 'CLI'), doc('api-reference', 'API')],
    },
  ],
};

export default sidebars;
