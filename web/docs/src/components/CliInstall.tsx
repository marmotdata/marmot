import React from "react";
import CodeBlock from "@theme/CodeBlock";
import { Steps, Step, Tabs, TabPanel, TipBox } from "./Steps";

/**
 * Shared installation block used by the CLI Reference and Deploy / CLI pages.
 * Update this component to change install instructions everywhere at once.
 */
export function CliInstall(): JSX.Element {
  return (
    <Tabs
      items={[
        { label: "Automatic", value: "auto" },
        { label: "Manual", value: "manual" },
      ]}
    >
      <TabPanel>
        <h4 className="doc-subhead">Homebrew</h4>
        <CodeBlock language="bash">brew install marmot</CodeBlock>

        <h4 className="doc-subhead">Install script</h4>
        <CodeBlock language="bash">curl -fsSL get.marmotdata.io | sh</CodeBlock>

        <TipBox variant="info" title="Verify scripts">
          It&apos;s good practice to inspect the contents of any script before
          piping it into bash.
        </TipBox>
      </TabPanel>

      <TabPanel>
        <Steps>
          <Step title="Download the binary">
            Download the latest Marmot binary for your platform from{" "}
            <a
              href="https://github.com/marmotdata/marmot/releases"
              target="_blank"
              rel="noopener noreferrer"
            >
              GitHub Releases
            </a>
            .
          </Step>
          <Step title="Make it executable">
            <CodeBlock language="bash">chmod +x marmot</CodeBlock>
          </Step>
          <Step title="Move to your PATH">
            <CodeBlock language="bash">sudo mv marmot /usr/local/bin/</CodeBlock>
          </Step>
        </Steps>
      </TabPanel>
    </Tabs>
  );
}

export default CliInstall;
