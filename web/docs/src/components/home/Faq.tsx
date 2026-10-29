import React, { useEffect } from "react";

export interface FaqItem {
  q: string;
  a: string;
}

/** What people ask about Marmot itself, for the homepage. */
export const productFaq: FaqItem[] = [
  {
    q: 'What can an agent see and query?',
    a: 'Only what you grant it. An agent connects with its own service account, and Marmot checks every query against that account’s roles before it reaches your database. Anything it may not read is refused, and every attempt is recorded.',
  },
  {
    q: 'Do I have to move my data?',
    a: 'No. Plugins read metadata from the systems you already run, and queries go to those systems in place. Your data stays where it is.',
  },
  {
    q: 'Is this the same Marmot I can self-host?',
    a: 'It is the same project. Cloud instances run the version we maintain for customers: fixes land there first, and many flow back into the MIT-licensed releases on GitHub. Nothing locks you in, and you can export your catalog and self-host the open source edition at any time.',
  },
  {
    q: 'What data leaves my systems?',
    a: 'Only metadata: schemas, ownership, descriptions, lineage and run status. Plugins never send rows, messages or payloads to your instance.',
  },
  {
    q: 'Do I have to give Marmot my database credentials?',
    a: 'No. Register where the secret lives in Google Secret Manager, AWS Secrets Manager, Azure Key Vault or HashiCorp Vault, and your instance reads it when a run starts, over a five-minute OpenID Connect token your cloud trusts. The run record keeps a mask.',
  },
  {
    q: 'Where does our instance run?',
    a: 'You choose the region when you launch, and your catalog stays there. Every instance runs its own Marmot with its own database, so catalogs are never shared between customers.',
  },
  {
    q: 'Who handles upgrades?',
    a: 'You upgrade Marmot when it suits you, and fixes reach Cloud before the open source releases. Everything underneath is patched continuously, with nothing for you to do.',
  },
  {
    q: 'Who runs Marmot Cloud?',
    a: 'The people who build Marmot. The team includes engineers from HashiCorp, Adidas, Just Eat Takeaway.com and Traefik, who help maintain Kubernetes, Terraform, Redpanda and Cloud Native Computing Foundation projects.',
  },
];

/** What people ask about plans and billing, for the pricing page. */
export const pricingFaq: FaqItem[] = [
  {
    q: 'Is there a free plan?',
    a: 'Yes. Creating an account is free and needs no card. Launch your first instance on the Free plan and move to Team or Scale when you outgrow it. A Free instance pauses when it sits idle, and you can start it again from the console. Paid plans are always on.',
  },
  {
    q: 'What is a compute unit?',
    a: 'The capacity your instance runs on. Each unit holds 5,000 assets and gives Marmot a dedicated vCPU and 2 GiB of memory, 100 requests a second, 40 Mbit/s of bandwidth and 25 GiB of database storage. Compute and requests burst to twice that, so a spike lands on headroom you already have. You need enough units for your catalog or your agent traffic, whichever is bigger. Team holds 1 to 3 units, and Scale 4 or more, at the same $595 per unit.',
  },
  {
    q: 'Why don’t you charge per user?',
    a: 'People browsing a catalog cost almost nothing to serve. What costs compute is the size of your catalog and how hard your agents query it, so that is what units measure, and users are unlimited on every plan.',
  },
  {
    q: 'Can I change units later?',
    a: 'Yes. Add units when the catalog grows or your agents get busier, and remove them when things quieten down. Changes are prorated on the next invoice, and the console shows how much of each unit your instance is using.',
  },
  {
    q: 'What does Enterprise add?',
    a: 'Several instances, a per-agent audit trail with SIEM export, VPC peering and data residency, custom backup retention, plugin development on request and a named support contact. Pricing is agreed with you.',
  },
  {
    q: 'What happens if I cancel?',
    a: 'Your instances keep running until the end of the billing period. After that they are stopped, and you have 30 days to export before they are deleted.',
  },
];

export default function Faq({ items = productFaq }: { items?: FaqItem[] }): JSX.Element {
  useEffect(() => {
    void import("@marmotdata/elements/disclosure.js");
  }, []);
  return (
    <mm-accordion className="mkt-faq">
      {items.map((item) => (
        <mm-disclosure key={item.q} summary={item.q} heading-level="3">
          <p>{item.a}</p>
        </mm-disclosure>
      ))}
    </mm-accordion>
  );
}
