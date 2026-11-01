import React, { useId, useState } from "react";
import CodeBlock from "@theme/CodeBlock";
import ExpandMore from "~icons/material-symbols/expand-more";

interface CollapsibleProps {
  title: string;
  /** Kept for existing content; not drawn. */
  icon?: string;
  children?: React.ReactNode;
  defaultOpen?: boolean;
  // For IAM policy support
  policyJson?: object;
  minimalPolicyJson?: object;
}

/** A section that folds away: one ruled panel with the title as its button. */
export function Collapsible({
  title,
  children,
  defaultOpen = false,
  policyJson,
  minimalPolicyJson,
}: CollapsibleProps): JSX.Element {
  const [isOpen, setIsOpen] = useState(defaultOpen);
  const [showMinimal, setShowMinimal] = useState(false);
  const panelId = useId();

  const currentPolicy = showMinimal && minimalPolicyJson ? minimalPolicyJson : policyJson;

  return (
    <div className="doc-collapsible">
      <button
        type="button"
        className="doc-collapsible__toggle"
        aria-expanded={isOpen}
        aria-controls={panelId}
        onClick={() => setIsOpen(!isOpen)}
      >
        <span>{title}</span>
        <ExpandMore aria-hidden="true" className="doc-collapsible__caret" />
      </button>
      {isOpen && (
        <div className="doc-collapsible__body" id={panelId}>
          {policyJson ? (
            <>
              {minimalPolicyJson && (
                <div className="mm-seg doc-collapsible__seg" role="group" aria-label="Policy">
                  <button type="button" aria-pressed={!showMinimal} onClick={() => setShowMinimal(false)}>
                    Full permissions
                  </button>
                  <button type="button" aria-pressed={showMinimal} onClick={() => setShowMinimal(true)}>
                    Minimal
                  </button>
                </div>
              )}
              <CodeBlock language="json">{JSON.stringify(currentPolicy, null, 2)}</CodeBlock>
            </>
          ) : (
            children
          )}
        </div>
      )}
    </div>
  );
}
