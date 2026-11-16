import React from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import "./style.css";
import "./agent-access.css";
import { ConsoleConfigProvider } from "./console-config";
import { ConfirmHost } from "./confirm-dialog";
import { queryClient } from "./query";
import { App } from "./app";
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <ConsoleConfigProvider>
        <App />
        <ConfirmHost />
      </ConsoleConfigProvider>
    </QueryClientProvider>
  </React.StrictMode>,
);
