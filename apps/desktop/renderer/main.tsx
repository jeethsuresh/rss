import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { WebApp } from "./WebApp";
import "./styles/global.css";

import { registerPWA } from "./lib/pwa";
if (!window.rss) registerPWA();

const RootApp = window.rss ? App : WebApp;

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <RootApp />
  </StrictMode>,
);
