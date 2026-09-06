import React from "react";
import ReactDOM from "react-dom/client";
import "@fontsource-variable/inter";
import "@phosphor-icons/web/regular";
import "@phosphor-icons/web/light";
import App from "./App";
import { StartupUpdate } from "./screens/UpdatePrompt";
import { CallOverlay } from "./shell/CallOverlay";
import "./styles/index.css";

type TauriMetadata = { metadata?: { currentWindow?: { label?: string } } };

const label = (window as unknown as { __TAURI_INTERNALS__?: TauriMetadata })
  .__TAURI_INTERNALS__?.metadata?.currentWindow?.label;

const overlay =
  label === "overlay" || new URLSearchParams(window.location.search).has("overlay");
if (overlay) document.documentElement.classList.add("is-overlay");

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    {overlay ? (
      <CallOverlay />
    ) : (
      <>
        <App />
        <StartupUpdate />
      </>
    )}
  </React.StrictMode>,
);
