import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { WebApp } from "./WebApp";
import "./styles/global.css";

const RootApp = window.rss ? App : WebApp;

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <RootApp />
  </StrictMode>,
);
