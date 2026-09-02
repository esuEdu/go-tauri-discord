import React from "react";
import ReactDOM from "react-dom/client";
import "@fontsource-variable/inter";
import "@phosphor-icons/web/regular";
import "@phosphor-icons/web/light";
import App from "./App";
import { StartupUpdate } from "./screens/UpdatePrompt";
import "./styles/index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
    <StartupUpdate />
  </React.StrictMode>,
);
