---
sidebar_position: 3.5
title: Use your own domain
description: Serve a Marmot Cloud instance on a hostname you control, alongside its generated one.
---

import { Steps, Step } from '@site/src/components/Steps';
import { ThemedImg } from '@site/src/components/ThemedImg';

# Use your own domain

Serve your instance on a hostname of your own, such as `marmot.example.com`, alongside its generated `marmotdata.cloud` one.

## Before you start

- Your account is on Scale or Enterprise.
- You can add DNS records for the hostname at your DNS provider.
- The hostname is a subdomain, such as `marmot.example.com`. A bare domain like `example.com` can't hold a CNAME record.

You'll add two records, and the console shows the exact values for each:

| Record | Name | Points to | Why |
| --- | --- | --- | --- |
| TXT | Given by the console | A one-time token | Proves you control this one hostname |
| CNAME | `marmot.example.com` | `<instance>.marmotdata.cloud` | Sends traffic to your instance |

## Add the domain

Everything happens on the instance's page in the console.

<Steps>
<Step title="Prove you control the hostname">

1. Under **Custom domain**, select **Add custom domain** and enter `marmot.example.com`.
2. Select **Get TXT record** and add that record at your DNS provider.
3. Select **Verify hostname**.

</Step>
<Step title="Point it at your instance">

1. Add the CNAME record the console shows.
2. Select **Check DNS**, then **Save hostname**.

</Step>
</Steps>

The TXT record step for `marmot.example.com`:

<ThemedImg
  lightSrc="/img/cloud-console-custom-domain-light.png"
  darkSrc="/img/cloud-console-custom-domain-dark.png"
  alt="The Custom domain section at the Prove it is yours step, showing the TXT record name and value for marmot.example.com and the Verify hostname button"
  size="panel"
/>

## Check that it worked

The domain moves through **Waiting for DNS**, **Issuing certificate** and **Active**. The certificate usually takes a few minutes, and the generated hostname keeps working throughout.

Open `https://marmot.example.com` once it shows **Active**.

## Remove the domain

Select **Remove custom domain**. The instance stops answering on it, and the generated hostname keeps working.

## Next steps

- [Access control](access-control.md): decide who can see and change what.
- [Set up single sign-on](single-sign-on.md): let your team sign in through your identity provider.
