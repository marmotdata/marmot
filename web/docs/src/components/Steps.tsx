import React from "react";
import { Callout } from "./ui/Callout";

interface StepProps {
  title: string;
  children: React.ReactNode;
}

export function Step({ title, children }: StepProps): JSX.Element {
  return (
    <>
      <h4 className="doc-steps__title">{title}</h4>
      <div className="doc-steps__body">{children}</div>
    </>
  );
}

interface StepsProps {
  children: React.ReactNode;
}

/** Numbered steps: a quiet number and a hairline joining one step to the next. */
export function Steps({ children }: StepsProps): JSX.Element {
  const steps = React.Children.toArray(children);
  return (
    <ol className="doc-steps">
      {steps.map((step, index) => (
        <li key={index} className="doc-steps__item">
          <span className="doc-steps__n" aria-hidden="true">
            {index + 1}
          </span>
          <div className="doc-steps__content">{step}</div>
        </li>
      ))}
    </ol>
  );
}

interface TabItem {
  label: string;
  value: string;
  /** Kept for existing content; tabs are labelled by their words alone. */
  icon?: string;
}

interface TabsProps {
  items: TabItem[];
  children: React.ReactNode;
  /**
   * Tabs that share a `groupId` synchronise within a single page — pick one
   * tab and every other Tabs block on the same page with the same `groupId`
   * follows. State resets on navigation; nothing is persisted to localStorage.
   */
  groupId?: string;
}

interface TabSyncValue {
  values: Record<string, string>;
  setValue: (groupId: string, value: string) => void;
}

const TabSyncContext = React.createContext<TabSyncValue | null>(null);

/**
 * Provides shared tab state for any descendant `Tabs` blocks that pass a
 * matching `groupId`. Pass `resetKey` (e.g. the current page pathname) to
 * clear the shared state on navigation, so picks don't leak across pages.
 */
export function TabSyncProvider({
  children,
  resetKey,
}: {
  children?: React.ReactNode;
  resetKey?: string;
}): JSX.Element {
  const [values, setValues] = React.useState<Record<string, string>>({});
  React.useEffect(() => {
    setValues({});
  }, [resetKey]);
  const setValue = React.useCallback((groupId: string, value: string) => {
    setValues((prev) => (prev[groupId] === value ? prev : { ...prev, [groupId]: value }));
  }, []);
  const ctx = React.useMemo(() => ({ values, setValue }), [values, setValue]);
  return <TabSyncContext.Provider value={ctx}>{children}</TabSyncContext.Provider>;
}

// useLayoutEffect runs after DOM mutation but before paint — the only place
// to compensate for layout shifts without a visible flash. Fall back to
// useEffect during SSR to silence React's hydration warning.
const useIsomorphicLayoutEffect =
  typeof window !== "undefined" ? React.useLayoutEffect : React.useEffect;

/** The design system's tabs (mm-tabs): one Tab stop, arrow keys move between tabs. */
export function Tabs({ items, children, groupId }: TabsProps): JSX.Element {
  const initial = items[0]?.value || "";
  const ctx = React.useContext(TabSyncContext);
  const [localActive, setLocalActive] = React.useState(initial);
  const tabRefs = React.useRef(new Map<string, HTMLButtonElement | null>());
  const baseId = React.useId();
  // Set on click; consumed in the post-render layout effect to pin the
  // clicked tab to the same viewport position across the panel swap.
  const pendingAnchor = React.useRef<{ value: string; top: number } | null>(null);

  const useShared = groupId !== undefined && ctx !== null;
  const sharedActive = useShared ? ctx.values[groupId!] : undefined;
  // Fall back to the first item if the shared value isn't present in this
  // Tabs block (different groups can have different value sets).
  const activeTab = useShared
    ? items.some((i) => i.value === sharedActive)
      ? (sharedActive as string)
      : initial
    : localActive;

  const setActiveTab = (value: string) => {
    const tab = tabRefs.current.get(value);
    if (tab) {
      pendingAnchor.current = { value, top: tab.getBoundingClientRect().top };
    }
    if (useShared) ctx!.setValue(groupId!, value);
    else setLocalActive(value);
  };

  // After the panel swap, scroll by the delta so the tab the user clicked
  // stays under their cursor — even when synced Tabs blocks above resize
  // the layout out from under it.
  useIsomorphicLayoutEffect(() => {
    const anchor = pendingAnchor.current;
    if (!anchor) return;
    pendingAnchor.current = null;
    const tab = tabRefs.current.get(anchor.value);
    if (!tab) return;
    const delta = tab.getBoundingClientRect().top - anchor.top;
    if (delta !== 0) window.scrollBy(0, delta);
  }, [activeTab]);

  const onKeyDown = (e: React.KeyboardEvent, index: number) => {
    const last = items.length - 1;
    const next =
      e.key === "ArrowRight" ? (index === last ? 0 : index + 1)
      : e.key === "ArrowLeft" ? (index === 0 ? last : index - 1)
      : e.key === "Home" ? 0
      : e.key === "End" ? last
      : -1;
    if (next < 0) return;
    e.preventDefault();
    const value = items[next].value;
    setActiveTab(value);
    tabRefs.current.get(value)?.focus();
  };

  const childArray = React.Children.toArray(children);

  return (
    <div className="doc-tabs">
      <div className="mm-tabs doc-tabs__list" role="tablist">
        {items.map((item, index) => {
          const selected = activeTab === item.value;
          return (
            <button
              key={item.value}
              ref={(el) => {
                tabRefs.current.set(item.value, el);
              }}
              type="button"
              role="tab"
              id={`${baseId}-tab-${index}`}
              aria-selected={selected}
              aria-controls={`${baseId}-panel-${index}`}
              tabIndex={selected ? 0 : -1}
              className="mm-tab"
              onClick={() => setActiveTab(item.value)}
              onKeyDown={(e) => onKeyDown(e, index)}
            >
              {item.label}
            </button>
          );
        })}
      </div>
      {childArray.map((child, index) => (
        <div
          key={items[index]?.value || index}
          role="tabpanel"
          id={`${baseId}-panel-${index}`}
          aria-labelledby={`${baseId}-tab-${index}`}
          hidden={activeTab !== items[index]?.value}
          className="doc-tabs__panel"
        >
          {child}
        </div>
      ))}
    </div>
  );
}

interface TabPanelProps {
  children: React.ReactNode;
}

export function TabPanel({ children }: TabPanelProps): JSX.Element {
  return <>{children}</>;
}

interface TipBoxProps {
  variant?: "info" | "warning" | "success" | "danger";
  title?: string;
  children: React.ReactNode;
}

/** A callout inside docs content, the same as an admonition. */
export function TipBox({ variant = "info", title, children }: TipBoxProps): JSX.Element {
  return (
    <Callout tone={variant} title={title}>
      {children}
    </Callout>
  );
}

interface ConfigTableProps {
  children: React.ReactNode;
}

export function ConfigTable({ children }: ConfigTableProps): JSX.Element {
  return (
    <table className="doc-config-table">
      <thead>
        <tr>
          <th scope="col">Option</th>
          <th scope="col">Description</th>
          <th scope="col">Default</th>
        </tr>
      </thead>
      <tbody>{children}</tbody>
    </table>
  );
}

interface ConfigRowProps {
  option: string;
  description: string;
  defaultValue?: string;
}

export function ConfigRow({ option, description, defaultValue = "-" }: ConfigRowProps): JSX.Element {
  return (
    <tr>
      <td>
        <code>{option}</code>
      </td>
      <td>{description}</td>
      <td>
        <code>{defaultValue}</code>
      </td>
    </tr>
  );
}
