import React, { useState } from "react";
import { Icon } from "@iconify/react";
import CodeBlock from "@theme/CodeBlock";

interface CollapsibleProps {
  title: string;
  icon?: string;
  children?: React.ReactNode;
  defaultOpen?: boolean;
  // For IAM policy support
  policyJson?: object;
  minimalPolicyJson?: object;
}

export function Collapsible({
  title,
  icon,
  children,
  defaultOpen = false,
  policyJson,
  minimalPolicyJson,
}: CollapsibleProps): JSX.Element {
  const [isOpen, setIsOpen] = useState(defaultOpen);
  const [showMinimal, setShowMinimal] = useState(false);

  const currentPolicy = showMinimal && minimalPolicyJson ? minimalPolicyJson : policyJson;

  return (
    <div className="my-4 border-2 border-earthy-terracotta-200 dark:border-earthy-terracotta-800 rounded-lg overflow-hidden">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center justify-between px-4 py-4 bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900 hover:bg-earthy-terracotta-100 dark:hover:bg-earthy-terracotta-800 cursor-pointer border-none transition-colors"
      >
        <div className="flex items-center gap-3">
          {icon && (
            <Icon
              icon={icon}
              className="w-6 h-6 text-earthy-terracotta-500"
            />
          )}
          <span className="font-semibold text-base text-earthy-terracotta-800 dark:text-earthy-terracotta-100">
            {title}
          </span>
        </div>
        <Icon
          icon="mdi:chevron-down"
          className={`w-5 h-5 text-earthy-terracotta-500 transition-transform ${isOpen ? "rotate-180" : ""}`}
        />
      </button>
      {isOpen && (
        <div className="border-t border-earthy-terracotta-200 dark:border-earthy-terracotta-800">
          {policyJson ? (
            <div className="p-4">
              {minimalPolicyJson && (
                <div className="flex gap-2 mb-4">
                  <button
                    onClick={() => setShowMinimal(false)}
                    className={`px-3 py-1.5 text-sm font-medium rounded-lg border transition-colors cursor-pointer ${
                      !showMinimal
                        ? "bg-earthy-green-100 border-earthy-green-300 text-earthy-green-800"
                        : "bg-transparent border-gray-300 text-gray-600 hover:bg-gray-50"
                    }`}
                  >
                    Full Permissions
                  </button>
                  <button
                    onClick={() => setShowMinimal(true)}
                    className={`px-3 py-1.5 text-sm font-medium rounded-lg border transition-colors cursor-pointer ${
                      showMinimal
                        ? "bg-earthy-blue-100 border-earthy-blue-300 text-earthy-blue-800"
                        : "bg-transparent border-gray-300 text-gray-600 hover:bg-gray-50"
                    }`}
                  >
                    Minimal
                  </button>
                </div>
              )}
              <CodeBlock language="json">
                {JSON.stringify(currentPolicy, null, 2)}
              </CodeBlock>
            </div>
          ) : (
            <div className="p-5 space-y-4 text-gray-700 dark:text-gray-300 [&>p]:m-0 [&>pre]:m-0">
              {children}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
