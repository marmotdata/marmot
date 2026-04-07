import React from "react";
import Layout from "@theme/Layout";

export default function Privacy(): JSX.Element {
  return (
    <Layout
      title="Privacy Policy"
      description="Marmot privacy policy — how we collect, use, and protect your personal data."
    >
      <div className="bg-earthy-brown-50 dark:bg-gray-900 min-h-screen">
        <article className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-16 prose prose-sm dark:prose-invert prose-headings:font-bold prose-a:text-earthy-terracotta-700 dark:prose-a:text-earthy-terracotta-400">
          <h1>Privacy Policy</h1>
          <p className="text-gray-500 dark:text-gray-400 !mt-0">
            Last updated: 7 April 2026
          </p>

          <p>
            This Privacy Notice for <strong>Marmot Data</strong> ("we", "us", or
            "our") describes how and why we collect, store, use, and share your
            personal information when you use our website at{" "}
            <strong>marmotdata.io</strong> or engage with us in other related
            ways, including marketing communications.
          </p>
          <p>
            <strong>Questions or concerns?</strong> Contact us at{" "}
            <a href="mailto:privacy@marmotdata.io">privacy@marmotdata.io</a>.
          </p>

          {/* ------------------------------------------------------------ */}
          <h2 id="collect">1. What information do we collect?</h2>
          <p>
            We only collect personal information that you voluntarily provide to
            us. We do not use cookies, analytics, or any form of automatic
            tracking. The information we collect depends on how you interact with
            us:
          </p>
          <table>
            <thead>
              <tr>
                <th>Form</th>
                <th>Data collected</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td>Cloud waitlist</td>
                <td>Email address, marketing preference</td>
              </tr>
              <tr>
                <td>Contact / enquiry</td>
                <td>Name, email address, message content</td>
              </tr>
            </tbody>
          </table>
          <p>
            We do not collect sensitive personal information, and we do not
            collect any information from third parties.
          </p>

          {/* ------------------------------------------------------------ */}
          <h2 id="why">2. How and why do we process your information?</h2>
          <p>
            We process your personal information for the following purposes:
          </p>
          <ul>
            <li>
              <strong>To notify waitlist registrants when Marmot Cloud launches</strong>{" "}
              — fulfilling the purpose for which you signed up.
            </li>
            <li>
              <strong>To respond to enquiries</strong> — replying to messages you
              send us via the contact form.
            </li>
            <li>
              <strong>To send marketing and product updates</strong> — only if
              you have given us explicit consent by ticking the marketing
              checkbox. You can opt out at any time by clicking the unsubscribe
              link in any email, or by contacting us.
            </li>
          </ul>

          {/* ------------------------------------------------------------ */}
          <h2 id="legal">3. What legal bases do we rely on?</h2>
          <p>
            Under the UK GDPR and EU GDPR, we rely on the following legal bases:
          </p>
          <ul>
            <li>
              <strong>Consent</strong> — for sending marketing and promotional
              communications. You can{" "}
              <a href="#rights">withdraw your consent</a> at any time.
            </li>
            <li>
              <strong>Legitimate interests</strong> — for notifying waitlist
              registrants when the product launches, and for responding to
              enquiries you have sent us. These interests do not outweigh your
              rights and freedoms given the limited data involved and your
              reasonable expectations when submitting the forms.
            </li>
            <li>
              <strong>Legal obligations</strong> — where necessary for
              compliance with applicable law.
            </li>
          </ul>

          {/* ------------------------------------------------------------ */}
          <h2 id="sharing">
            4. Who processes your data?
          </h2>
          <p>
            We share your personal information with the following service
            providers who act as data processors on our behalf:
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
                  <a
                    href="https://www.cloudflare.com/privacypolicy/"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    Cloudflare
                  </a>
                </td>
                <td>API request processing</td>
                <td>Ireland (EU)</td>
              </tr>
              <tr>
                <td>
                  <a
                    href="https://resend.com/legal/privacy-policy"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    Resend
                  </a>
                </td>
                <td>Email delivery and contact list management</td>
                <td>United States</td>
              </tr>
            </tbody>
          </table>
          <p>
            We do not sell, rent, or share your personal information with any
            third party for their own marketing purposes. We may transfer your
            information in connection with a merger or acquisition of our
            business.
          </p>

          {/* ------------------------------------------------------------ */}
          <h2 id="transfers">5. International transfers</h2>
          <p>
            Your data is processed by Cloudflare Workers and stored by Resend
            in the United States for email delivery and contact list management.
          </p>
          <p>
            Where data is transferred outside the UK or EEA, we rely on the
            European Commission's Standard Contractual Clauses (SCCs) and the
            UK's International Data Transfer Agreement to ensure your personal
            information receives an adequate level of protection.
          </p>

          {/* ------------------------------------------------------------ */}
          <h2 id="retention">6. How long do we keep your information?</h2>
          <ul>
            <li>
              <strong>Waitlist contacts</strong> — retained until the product
              launches or for up to 12 months, whichever is sooner, then
              removed.
            </li>
            <li>
              <strong>Newsletter subscribers</strong> — retained until you
              unsubscribe.
            </li>
            <li>
              <strong>Contact enquiries</strong> — used only to send a
              notification email and not stored beyond delivery.
            </li>
          </ul>
          <p>
            When we have no ongoing legitimate business need to process your
            personal information, we will delete or anonymise it.
          </p>

          {/* ------------------------------------------------------------ */}
          <h2 id="security">7. How do we keep your information safe?</h2>
          <p>
            We have implemented appropriate technical and organisational security
            measures, including encrypted connections (TLS), access controls, and
            EU-region data storage. However, no electronic transmission over the
            internet can be guaranteed to be 100% secure.
          </p>

          {/* ------------------------------------------------------------ */}
          <h2 id="children">8. Children</h2>
          <p>
            We do not knowingly collect data from or market to anyone under 18
            years of age. If we learn that we have collected personal information
            from a child, we will promptly delete it. Please contact us at{" "}
            <a href="mailto:privacy@marmotdata.io">privacy@marmotdata.io</a> if
            you believe we have inadvertently collected such data.
          </p>

          {/* ------------------------------------------------------------ */}
          <h2 id="rights">9. Your rights</h2>
          <p>
            Depending on your location, you may have the following rights under
            applicable data protection laws (including UK GDPR, EU GDPR, CCPA,
            and other regional laws):
          </p>
          <ul>
            <li>
              <strong>Access</strong> — request a copy of the personal
              information we hold about you.
            </li>
            <li>
              <strong>Rectification</strong> — request correction of inaccurate
              data.
            </li>
            <li>
              <strong>Erasure</strong> — request deletion of your personal
              information.
            </li>
            <li>
              <strong>Restriction</strong> — request that we limit how we use
              your data.
            </li>
            <li>
              <strong>Portability</strong> — receive your data in a structured,
              machine-readable format.
            </li>
            <li>
              <strong>Objection</strong> — object to processing based on
              legitimate interests.
            </li>
            <li>
              <strong>Withdraw consent</strong> — where we rely on consent (e.g.
              marketing), you can withdraw it at any time without affecting the
              lawfulness of prior processing.
            </li>
          </ul>
          <p>
            To exercise any of these rights, email us at{" "}
            <a href="mailto:privacy@marmotdata.io">privacy@marmotdata.io</a>. We
            will respond within one month (or sooner where required by law).
          </p>
          <p>
            We do not sell your personal data, do not engage in targeted
            advertising, and do not profile users. US residents have the right to
            non-discrimination for exercising their privacy rights.
          </p>
          <p>
            If you believe we are unlawfully processing your personal
            information, you have the right to lodge a complaint with your local
            data protection authority. In the UK, this is the{" "}
            <a
              href="https://ico.org.uk/make-a-complaint/data-protection-complaints/data-protection-complaints/"
              target="_blank"
              rel="noopener noreferrer"
            >
              Information Commissioner's Office (ICO)
            </a>
            .
          </p>

          {/* ------------------------------------------------------------ */}
          <h2 id="updates">10. Updates to this notice</h2>
          <p>
            We may update this Privacy Notice from time to time. The updated
            version will be indicated by a revised "Last updated" date at the top
            of this page. If we make material changes, we will notify you by
            prominently posting a notice on our website.
          </p>

          {/* ------------------------------------------------------------ */}
          <h2 id="contact">11. Contact us</h2>
          <p>
            If you have any questions about this Privacy Notice or wish to
            exercise your rights, contact us at:
          </p>
          <p>
            <strong>Marmot Data</strong>
            <br />
            Email:{" "}
            <a href="mailto:privacy@marmotdata.io">privacy@marmotdata.io</a>
            <br />
            United Kingdom
          </p>
        </article>
      </div>
    </Layout>
  );
}
