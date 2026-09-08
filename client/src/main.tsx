import React from "react";
import ReactDOM from "react-dom/client";
import "@fontsource-variable/inter";
import "@phosphor-icons/web/regular";
import "@phosphor-icons/web/light";
import App from "./App";
import { StartupUpdate } from "./screens/UpdatePrompt";
import { watchInviteLinks } from "./deeplink";
import "./styles/index.css";

void watchInviteLinks();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
    <StartupUpdate />
  </React.StrictMode>,
);
