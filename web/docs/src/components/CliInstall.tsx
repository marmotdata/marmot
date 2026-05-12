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
        { label: "Automatic", value: "auto", icon: "mdi:download" },
        { label: "Manual", value: "manual", icon: "mdi:folder-download" },
      ]}
    >
      <TabPanel>
        <h4 className="text-base font-semibold text-gray-900 dark:text-white m-0 mb-2 mt-1">
          Homebrew
        </h4>
        <CodeBlock language="bash">brew install marmot</CodeBlock>

        <h4 className="text-base font-semibold text-gray-900 dark:text-white m-0 mb-2 mt-4">
          Install script
        </h4>
        <CodeBlock language="bash">curl -fsSL get.marmotdata.io | sh</CodeBlock>

        <TipBox variant="info" title="Verify Scripts">
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
