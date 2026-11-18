import type { ReactNode } from "react";
import { Card } from "./card";
import { cx } from "./utils";

// A signed-out or onboarding screen (sign in, set your own password): one
// card centred on the canvas, outside the console shell. The mark, eyebrow
// and title introduce the step; children are its form; the footer holds
// secondary actions and notes (single sign-on, sign out).
export function AuthLayout({
  title,
  description,
  mark,
  eyebrow,
  footer,
  children,
  className,
  cardClassName,
}: {
  title: ReactNode;
  description?: ReactNode;
  // A decorative icon above the title (lucide element).
  mark?: ReactNode;
  eyebrow?: ReactNode;
  footer?: ReactNode;
  children: ReactNode;
  className?: string;
  cardClassName?: string;
}) {
  return (
    <main className={cx("ui-auth", className)}>
      <Card className={cx("ui-auth-card", cardClassName)}>
        <header className="ui-auth-head">
          {mark && (
            <span className="ui-auth-mark" aria-hidden>
              {mark}
            </span>
          )}
          {eyebrow && <span className="ui-auth-eyebrow">{eyebrow}</span>}
          <h1>{title}</h1>
          {description && <p>{description}</p>}
        </header>
        {children}
        {footer && <footer className="ui-auth-footer">{footer}</footer>}
      </Card>
    </main>
  );
}
