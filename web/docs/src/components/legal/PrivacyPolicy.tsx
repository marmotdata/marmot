import React from "react";
import Link from "@docusaurus/Link";

export default function PrivacyPolicy(): JSX.Element {
  return (
    <>
      <p>
        This Privacy Notice for <strong>Marmot Data Ltd</strong> ("we", "us", or "our") describes
        how and why we collect, store, use, and share your personal information when you use our
        website at <strong>marmotdata.io</strong>, use Marmot Cloud
        at <strong>cloud.marmotdata.io</strong>, or engage with us in other related ways, including
        marketing communications.
      </p>
      <p>
        <strong>Questions or concerns?</strong> Contact us
        at <a href="mailto:privacy@marmotdata.io">privacy@marmotdata.io</a>.
      </p>
      <h2 id="collect">1. What information do we collect?</h2>
      <p>
        We collect personal information that you provide to us and, when you use Marmot Cloud,
        information about how you use it. We do not use third-party analytics, advertising cookies,
        or any form of cross-site tracking. The information we collect depends on how you interact
        with us:
      </p>
      <table>
        <thead>
          <tr>
            <th>How you interact with us</th>
            <th>Data collected</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>Marmot Cloud account</td>
            <td>
              Name, email address, sign-in identifiers, and your role and phone number if you give
              them
            </td>
          </tr>
          <tr>
            <td>Your organisation and billing</td>
            <td>
              Company name, size, country, billing address, VAT number, and website. Payment card
              details are collected by Stripe and we never see your full card number.
            </td>
          </tr>
          <tr>
            <td>Using Marmot Cloud</td>
            <td>
              Which version of our Terms you accepted, when, and the IP address and browser you
              accepted from, your email preferences and marketing consent, when you last used the
              console, and the instances and usage on your account
            </td>
          </tr>
          <tr>
            <td>Product analytics</td>
            <td>
              A daily record of your organisation's name, plan, compute units, size, country and
              website, how many members, invitations, instances and verified domains it has, and the
              plan, version and status of each instance. For each person, the role and preferences
              you set and when you signed up and last used the console, recorded against your
              account ID, not your name or email address
            </td>
          </tr>
          <tr>
            <td>Team invitations</td>
            <td>The email address of the person invited, and who invited them</td>
          </tr>
          <tr>
            <td>Security logs</td>
            <td>IP address, browser information, and the time of each request to Marmot Cloud</td>
          </tr>
          <tr>
            <td>Emails to us</td>
            <td>Name, email address, message content</td>
          </tr>
          <tr>
            <td>Cloud waitlist (now closed)</td>
            <td>Email address, marketing preference</td>
          </tr>
        </tbody>
      </table>
      <p>
        Marmot Cloud uses only the cookies and browser storage that are strictly necessary to keep
        you signed in and to protect the service. The marmotdata.io website does not set cookies.
      </p>
      <p>
        We do not collect sensitive personal information, and we do not collect any information
        about you from third parties, other than payment status from Stripe.
      </p>
      <p>
        <strong>Data in your catalog.</strong> The metadata and other data your organisation puts
        into a Marmot Cloud instance is processed by us on your organisation's behalf, under
        our <Link to="/dpa">Data Processing Agreement</Link>. Your organisation decides how that
        data is used, and this notice does not cover it.
      </p>
      <h2 id="why">2. How and why do we process your information?</h2>
      <p>We process your personal information for the following purposes:</p>
      <ul>
        <li>
          <strong>To provide Marmot Cloud</strong> — creating and managing your account and
          organisation, running your instances, taking payment, and recording your acceptance of our
          Terms.
        </li>
        <li>
          <strong>To send service emails</strong> — such as sign-in codes, invitations, billing
          receipts, trial reminders, and security notices. These are part of the service and you
          cannot opt out of them while you have an account.
        </li>
        <li>
          <strong>To keep Marmot Cloud secure</strong> — preventing abuse, fraud, and unauthorised
          access.
        </li>
        <li><strong>To respond to enquiries</strong> — replying to messages you send us.</li>
        <li>
          <strong>To understand and improve Marmot Cloud</strong> — reporting on how the service is
          used, such as how many organisations and people use it, which plans they are on, and how
          instances are sized, from the product analytics above.
        </li>
        <li>
          <strong>To send product updates</strong> — news about changes to Marmot Cloud, sent to
          account holders. You can turn these off at any time from your account page or by clicking
          the unsubscribe link in any email.
        </li>
        <li>
          <strong>To send marketing</strong> — only if you have given us explicit consent by ticking
          the marketing checkbox. You can opt out at any time by clicking the unsubscribe link in
          any email, from your account page, or by contacting us.
        </li>
      </ul>
      <h2 id="legal">3. What legal bases do we rely on?</h2>
      <p>Under the UK GDPR and EU GDPR, we rely on the following legal bases:</p>
      <ul>
        <li>
          <strong>Contract</strong> — for providing Marmot Cloud, managing your account, billing,
          and sending service emails.
        </li>
        <li>
          <strong>Consent</strong> — for sending marketing and promotional communications. You
          can <a href="#rights">withdraw your consent</a> at any time.
        </li>
        <li>
          <strong>Legitimate interests</strong> — for keeping Marmot Cloud secure, sending product
          updates to account holders, responding to enquiries you have sent us, and our product
          analytics. These interests do not outweigh your rights and freedoms given the limited data
          involved and your reasonable expectations when using our services.
        </li>
        <li>
          <strong>Legal obligations</strong> — where necessary for compliance with applicable law,
          such as keeping tax and accounting records.
        </li>
      </ul>
      <h2 id="sharing">4. Who processes your data?</h2>
      <p>
        We share your personal information with the following service providers who act as data
        processors on our behalf:
      </p>
      <table>
        <thead>
          <tr>
            <th>Provider</th>
            <th>Purpose</th>
            <th>Location</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>
              <a href="https://cloud.google.com/terms/cloud-privacy-notice" target="_blank" rel="noopener noreferrer">Google
              Cloud</a>
            </td>
            <td>Hosting Marmot Cloud, its databases, and our product analytics</td>
            <td>United Kingdom</td>
          </tr>
          <tr>
            <td>
              <a href="https://www.cloudflare.com/privacypolicy/" target="_blank" rel="noopener noreferrer">Cloudflare</a>
            </td>
            <td>Website and console traffic, DNS, and protection against attacks</td>
            <td>Global network; Ireland (EU)</td>
          </tr>
          <tr>
            <td>
              <a href="https://resend.com/legal/privacy-policy" target="_blank" rel="noopener noreferrer">Resend</a>
            </td>
            <td>Email delivery and contact list management</td>
            <td>United States</td>
          </tr>
        </tbody>
      </table>
      <p>
        <strong>Stripe</strong> processes your payments as an independent controller under its
        own <a href="https://stripe.com/privacy" target="_blank" rel="noopener noreferrer">privacy
        policy</a>. On marmotdata.io, search is provided by Algolia, which receives your search
        terms, and blog comments are provided by GitHub through giscus, under GitHub's own privacy
        policy if you choose to comment.
      </p>
      <p>
        We do not sell, rent, or share your personal information with any third party for their own
        marketing purposes. If you are a member of an organisation on Marmot Cloud, other members of
        that organisation can see your name and email address. We may transfer your information in
        connection with a merger or acquisition of our business.
      </p>
      <h2 id="transfers">5. International transfers</h2>
      <p>
        Marmot Cloud is hosted in the United Kingdom. Your data is processed by Cloudflare on its
        global network, and stored by Resend in the United States for email delivery and contact
        list management.
      </p>
      <p>
        Where data is transferred outside the UK or EEA, we rely on the European Commission's
        Standard Contractual Clauses (SCCs) and the UK's International Data Transfer Agreement to
        ensure your personal information receives an adequate level of protection.
      </p>
      <h2 id="retention">6. How long do we keep your information?</h2>
      <ul>
        <li>
          <strong>Marmot Cloud accounts</strong> — retained while your account is open. When you
          delete your account, we delete your personal information promptly, apart from the records
          below.
        </li>
        <li>
          <strong>Product analytics</strong> — kept as history. Once your account is deleted, the
          records about you can no longer be linked to you, as they carry only your account ID.
        </li>
        <li>
          <strong>Billing records</strong> — retained for six years after the end of the financial
          year they relate to, as UK tax law requires.
        </li>
        <li><strong>Security logs</strong> — retained for a short period and then deleted.</li>
        <li>
          <strong>Waitlist contacts</strong> — retained until the product launches or for up to 12
          months, whichever is sooner, then removed.
        </li>
        <li><strong>Newsletter subscribers</strong> — retained until you unsubscribe.</li>
        <li>
          <strong>Emails to us</strong> — retained for as long as needed to deal with your enquiry.
        </li>
      </ul>
      <p>
        When we have no ongoing legitimate business need to process your personal information, we
        will delete or anonymise it.
      </p>
      <h2 id="security">7. How do we keep your information safe?</h2>
      <p>
        We have implemented appropriate technical and organisational security measures, including
        encrypted connections (TLS), encryption at rest, access controls, and UK-region data
        storage. However, no electronic transmission over the internet can be guaranteed to be 100%
        secure. Our <Link to="/security">security page</Link> has more detail.
      </p>
      <h2 id="children">8. Children</h2>
      <p>
        We do not knowingly collect data from or market to anyone under 18 years of age. If we learn
        that we have collected personal information from a child, we will promptly delete it. Please
        contact us at <a href="mailto:privacy@marmotdata.io">privacy@marmotdata.io</a> if you
        believe we have inadvertently collected such data.
      </p>
      <h2 id="rights">9. Your rights</h2>
      <p>
        Depending on your location, you may have the following rights under applicable data
        protection laws (including UK GDPR, EU GDPR, CCPA, and other regional laws):
      </p>
      <ul>
        <li>
          <strong>Access</strong> — request a copy of the personal information we hold about you.
        </li>
        <li><strong>Rectification</strong> — request correction of inaccurate data.</li>
        <li><strong>Erasure</strong> — request deletion of your personal information.</li>
        <li><strong>Restriction</strong> — request that we limit how we use your data.</li>
        <li>
          <strong>Portability</strong> — receive your data in a structured, machine-readable format.
        </li>
        <li><strong>Objection</strong> — object to processing based on legitimate interests.</li>
        <li>
          <strong>Withdraw consent</strong> — where we rely on consent (e.g. marketing), you can
          withdraw it at any time without affecting the lawfulness of prior processing.
        </li>
      </ul>
      <p>
        You can update your details, change your email preferences, and delete your account yourself
        from your account page in Marmot Cloud. To exercise any of these rights, email us
        at <a href="mailto:privacy@marmotdata.io">privacy@marmotdata.io</a>. We will respond within
        one month (or sooner where required by law).
      </p>
      <p>
        We do not sell your personal data, do not engage in targeted advertising, and do not profile
        users. US residents have the right to non-discrimination for exercising their privacy
        rights.
      </p>
      <p>
        If you believe we are unlawfully processing your personal information, you have the right to
        lodge a complaint with your local data protection authority. In the UK, this is
        the <a href="https://ico.org.uk/make-a-complaint/data-protection-complaints/data-protection-complaints/" target="_blank" rel="noopener noreferrer">Information
        Commissioner's Office (ICO)</a>.
      </p>
      <h2 id="updates">10. Updates to this notice</h2>
      <p>
        We may update this Privacy Notice from time to time. The updated version will be indicated
        by a revised "Last updated" date at the top of this page. If we make material changes, we
        will notify you by prominently posting a notice on our website and, if you have a Marmot
        Cloud account, by email.
      </p>
      <h2 id="contact">11. Contact us</h2>
      <p>
        If you have any questions about this Privacy Notice or wish to exercise your rights, contact
        us at:
      </p>
      <p>
        <strong>Marmot Data Ltd</strong>
        <br />
        Company number 17420684
        <br />
        66 Paul Street, London, EC2A 4NA, United Kingdom
        <br />
        Email: <a href="mailto:privacy@marmotdata.io">privacy@marmotdata.io</a>
      </p>
    </>
  );
}
