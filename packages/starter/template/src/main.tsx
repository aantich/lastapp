import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { Lastapp } from "./lastapp";
import { Button, Panel } from "./ui";
import "./style.css";
declare global {
  interface Window {
    lastapp?: Lastapp;
  }
}
function App() {
  const [client, setClient] = useState<Lastapp>(),
    [message, setMessage] = useState(""),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    fetch("/_lastapp/config.json", { cache: "no-store" })
      .then((r) => {
        if (!r.ok) throw new Error("App configuration unavailable");
        return r.json();
      })
      .then((config) => {
        const sdk = new Lastapp(config.apiOrigin, config.appID);
        window.lastapp = sdk;
        setClient(sdk);
      })
      .catch((e) => setMessage(e.message));
  }, []);
  async function connect() {
    if (!client) return;
    setBusy(true);
    try {
      await client.connect();
      setMessage("Connected. Replace this page with your micro-apps.");
    } catch (e) {
      setMessage(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <main>
      <h1>Your App</h1>
      <p>One App backend, reusable entities, shared components.</p>
      <Panel title="Get started">
        <p>
          Inspect the published entity contracts before adding your business
          screens. Put reusable UI in ui.tsx and keep one shared stylesheet.
        </p>
        <Button disabled={!client || busy} onClick={connect}>
          {busy ? "Connecting…" : "Connect to Lastapp"}
        </Button>
        {message && <p role="status">{message}</p>}
      </Panel>
    </main>
  );
}
createRoot(document.getElementById("root")!).render(<App />);
