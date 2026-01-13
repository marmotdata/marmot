import React from "react";
import { Icon } from "@iconify/react";

interface StepProps {
  title: string;
  children: React.ReactNode;
}

export function Step({ title, children }: StepProps): JSX.Element {
  return (
    <div className="step-content">
      <h4 className="text-base font-semibold text-gray-900 dark:text-white m-0 mb-3">
        {title}
      </h4>
      <div className="text-gray-700 dark:text-gray-300 [&>p]:m-0 [&>p]:mb-3 [&>pre]:my-3 [&>ul]:my-2 [&>ol]:my-2">
        {children}
      </div>
    </div>
  );
}

interface StepsProps {
  children: React.ReactNode;
}

export function Steps({ children }: StepsProps): JSX.Element {
  const steps = React.Children.toArray(children);

  return (
    <div className="my-6">
      {steps.map((step, index) => (
        <div key={index} className="flex gap-4 pb-6 last:pb-0">
          {/* Step number column */}
          <div className="flex flex-col items-center">
            <div className="flex-shrink-0 w-8 h-8 rounded-full bg-[var(--ifm-color-primary)] text-white flex items-center justify-center font-bold text-sm">
              {index + 1}
            </div>
            {index < steps.length - 1 && (
              <div className="w-0.5 flex-1 bg-gray-200 dark:bg-gray-700 mt-2" />
            )}
          </div>
          {/* Step content column */}
          <div className="flex-1 min-w-0 pt-0.5">
            {step}
          </div>
        </div>
      ))}
    </div>
  );
}

interface TabItem {
  label: string;
  value: string;
  icon?: string;
}

interface TabsProps {
  items: TabItem[];
  children: React.ReactNode;
}

export function Tabs({ items, children }: TabsProps): JSX.Element {
  const [activeTab, setActiveTab] = React.useState(items[0]?.value || "");
  const childArray = React.Children.toArray(children);

  return (
    <div className="my-4">
      <div className="flex border-b border-gray-200 dark:border-gray-700 mb-4 overflow-x-auto">
        {items.map((item) => (
          <a
            key={item.value}
            onClick={(e) => {
              e.preventDefault();
              setActiveTab(item.value);
            }}
            href="#"
            className={`flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 transition-colors cursor-pointer whitespace-nowrap no-underline hover:no-underline ${
              activeTab === item.value
                ? "border-[var(--ifm-color-primary)] text-[var(--ifm-color-primary)]"
                : "border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
            }`}
          >
            {item.icon && <Icon icon={item.icon} className="w-4 h-4" />}
            {item.label}
          </a>
        ))}
      </div>
      <div>
        {childArray.map((child, index) => (
          <div
            key={items[index]?.value || index}
            className={activeTab === items[index]?.value ? "block" : "hidden"}
          >
            {child}
          </div>
        ))}
      </div>
    </div>
  );
}

interface TabPanelProps {
  children: React.ReactNode;
}

export function TabPanel({ children }: TabPanelProps): JSX.Element {
  return <div className="[&>pre]:my-0 [&>p]:mb-3">{children}</div>;
}

interface TipBoxProps {
  variant?: "info" | "warning" | "success" | "danger";
  title?: string;
  children: React.ReactNode;
}

export function TipBox({ variant = "info", title, children }: TipBoxProps): JSX.Element {
  const styles = {
    info: {
      bg: "bg-blue-50 dark:bg-blue-900/20",
      border: "border-blue-200 dark:border-blue-800",
      icon: "mdi:information",
      iconColor: "text-blue-500",
      titleColor: "text-blue-800 dark:text-blue-200",
    },
    warning: {
      bg: "bg-amber-50 dark:bg-amber-900/20",
      border: "border-amber-200 dark:border-amber-800",
      icon: "mdi:alert",
      iconColor: "text-amber-500",
      titleColor: "text-amber-800 dark:text-amber-200",
    },
    success: {
      bg: "bg-green-50 dark:bg-green-900/20",
      border: "border-green-200 dark:border-green-800",
      icon: "mdi:check-circle",
      iconColor: "text-green-500",
      titleColor: "text-green-800 dark:text-green-200",
    },
    danger: {
      bg: "bg-red-50 dark:bg-red-900/20",
      border: "border-red-200 dark:border-red-800",
      icon: "mdi:alert-circle",
      iconColor: "text-red-500",
      titleColor: "text-red-800 dark:text-red-200",
    },
  };

  const style = styles[variant];

  return (
    <div className={`my-4 p-4 rounded-lg border ${style.bg} ${style.border}`}>
      <div className="flex gap-3">
        <Icon icon={style.icon} className={`w-5 h-5 flex-shrink-0 mt-0.5 ${style.iconColor}`} />
        <div className="flex-1 min-w-0">
          {title && (
            <div className={`font-semibold text-sm mb-1 ${style.titleColor}`}>
              {title}
            </div>
          )}
          <div className="text-sm text-gray-700 dark:text-gray-300 [&>p]:m-0 [&>code]:text-xs">
            {children}
          </div>
        </div>
      </div>
    </div>
  );
}

interface ConfigTableProps {
  children: React.ReactNode;
}

export function ConfigTable({ children }: ConfigTableProps): JSX.Element {
  return (
    <div className="my-4 overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
      <table className="w-full text-sm">
        <thead className="bg-gray-50 dark:bg-gray-800">
          <tr>
            <th className="px-4 py-3 text-left font-semibold text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700">
              Option
            </th>
            <th className="px-4 py-3 text-left font-semibold text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700">
              Description
            </th>
            <th className="px-4 py-3 text-left font-semibold text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700">
              Default
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
          {children}
        </tbody>
      </table>
    </div>
  );
}

interface ConfigRowProps {
  option: string;
  description: string;
  defaultValue?: string;
}

export function ConfigRow({ option, description, defaultValue = "-" }: ConfigRowProps): JSX.Element {
  return (
    <tr className="bg-white dark:bg-gray-900">
      <td className="px-4 py-3 font-mono text-xs text-[var(--ifm-color-primary)]">
        {option}
      </td>
      <td className="px-4 py-3 text-gray-700 dark:text-gray-300">
        {description}
      </td>
      <td className="px-4 py-3 font-mono text-xs text-gray-500 dark:text-gray-400">
        {defaultValue}
      </td>
    </tr>
  );
}
