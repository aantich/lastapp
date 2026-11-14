import type { ButtonHTMLAttributes, PropsWithChildren } from "react";
// All micro-app pages import this shared UI module and the App's one stylesheet.
export function Button(props: ButtonHTMLAttributes<HTMLButtonElement>) {
  return <button {...props} />;
}
export function Panel({
  title,
  children,
}: PropsWithChildren<{ title: string }>) {
  return (
    <section className="panel">
      <h2>{title}</h2>
      {children}
    </section>
  );
}
