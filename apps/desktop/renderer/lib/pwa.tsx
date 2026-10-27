import { connectPush, disconnectPush, cleanupPush } from "./push";
import { useEffect, useState } from "react";
import "./pwa.css";
type InstallEvent = Event & {
  prompt(): Promise<void>;
  userChoice: Promise<{ outcome: string }>;
};
let installPrompt: InstallEvent | null = null;
let registration: ServiceWorkerRegistration | null = null;
let reloadForUpdate = false;
const preferences = "rss:device-notifications";
export function registerPWA() {
  window.addEventListener("beforeinstallprompt", (event) => {
    event.preventDefault();
    installPrompt = event as InstallEvent;
    window.dispatchEvent(new Event("pwa:change"));
  });
  window.addEventListener("appinstalled", () => {
    installPrompt = null;
    window.dispatchEvent(new Event("pwa:change"));
  });
  if (
    !import.meta.env.PROD ||
    !("serviceWorker" in navigator) ||
    !window.isSecureContext
  )
    return;
  navigator.serviceWorker.addEventListener("controllerchange", () => {
    if (reloadForUpdate) location.reload();
  });
  window.addEventListener("load", () => {
    void navigator.serviceWorker
      .register("/sw.js", { updateViaCache: "none" })
      .then((value) => {
        registration = value;
        value.addEventListener("updatefound", () =>
          value.installing?.addEventListener("statechange", () =>
            window.dispatchEvent(new Event("pwa:change")),
          ),
        );
        window.dispatchEvent(new Event("pwa:change"));
      })
      .catch(() => window.dispatchEvent(new Event("pwa:change")));
  });
}
export function DeviceFeatures() {
  const [, render] = useState(0);
  const [online, setOnline] = useState(navigator.onLine);
  const [message, setMessage] = useState("");
  const [enabled, setEnabled] = useState(() => {
    try {
      return localStorage.getItem(preferences) === "on";
    } catch {
      return false;
    }
  });
  useEffect(() => {
    const update = () => {
      setOnline(navigator.onLine);
      render((n) => n + 1);
    };
    const reconnect = () => {
      void cleanupPush().catch(() => {});
      if (enabled)
        void connectPush()
          .then((connected) => {
            if (!connected) {
              setEnabled(false);
              localStorage.removeItem(preferences);
            }
          })
          .catch((error) =>
            setMessage(
              error instanceof Error
                ? error.message
                : "Reconnect to refresh notifications.",
            ),
          );
    };
    reconnect();
    window.addEventListener("online", reconnect);
    const unavailable = () =>
      setMessage(
        "Device storage is unavailable or full. New data may not be available offline.",
      );
    window.addEventListener("rss:storage-unavailable", unavailable);
    window.addEventListener("online", update);
    window.addEventListener("offline", update);
    window.addEventListener("pwa:change", update);
    return () => {
      window.removeEventListener("rss:storage-unavailable", unavailable);
      window.removeEventListener("online", reconnect);
      window.removeEventListener("online", update);
      window.removeEventListener("offline", update);
      window.removeEventListener("pwa:change", update);
    };
  }, [enabled]);
  async function enableNotifications() {
    try {
      if (enabled) {
        await disconnectPush();
        setEnabled(false);
        return;
      }
      if (!("Notification" in window) || !("serviceWorker" in navigator)) {
        setMessage(
          "Notifications are unavailable in this browser. On iPhone or iPad, install this app to your Home Screen first.",
        );
        return;
      }
      const permission = await Notification.requestPermission();
      if (permission !== "granted") {
        setMessage(
          "Notifications are off. You can change permission in your browser settings.",
        );
        return;
      }
      await connectPush(true);
      localStorage.setItem(preferences, "on");
      setEnabled(true);
      setMessage("Notifications enabled on this device.");
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : "Notifications could not be enabled on this device.",
      );
    }
  }
  return (
    <details className="pwa-controls">
      <summary aria-label="App and offline settings">
        {online ? "App" : "Offline"}
      </summary>
      <div className="pwa-panel">
        <strong>News on this device</strong>
        <p role="status">
          {online ? "Connected" : "Offline — using saved data"}
        </p>
        <p>
          Visited feeds, articles, stories, and sports data are saved for
          offline use. Read status syncs when you reconnect.
        </p>
        {installPrompt ? (
          <button
            type="button"
            onClick={async () => {
              const prompt = installPrompt;
              if (!prompt) return;
              await prompt.prompt();
              await prompt.userChoice;
              installPrompt = null;
              render((n) => n + 1);
            }}
          >
            Install app
          </button>
        ) : (
          <p>
            Install from your browser menu, or use Share → Add to Home Screen on
            iPhone and iPad.
          </p>
        )}
        <button type="button" onClick={() => void enableNotifications()}>
          {enabled ? "Turn off notifications" : "Enable notifications"}
        </button>
        <small>
          New articles after all feeds refresh, at most once per day. Works when
          the app is closed.
        </small>
        <button
          type="button"
          onClick={async () => {
            try {
              const saved = await navigator.storage?.persist?.();
              setMessage(
                saved
                  ? "Device storage is protected from automatic cleanup."
                  : "This browser manages storage. Keep a separate backup of important data.",
              );
            } catch {
              setMessage("Persistent storage is unavailable in this browser.");
            }
          }}
        >
          Keep data on this device
        </button>
        {registration?.waiting && (
          <button
            type="button"
            onClick={() => {
              reloadForUpdate = true;
              registration?.waiting?.postMessage({ type: "ACTIVATE_UPDATE" });
            }}
          >
            Update app
          </button>
        )}
        {message && <p role="status">{message}</p>}
      </div>
    </details>
  );
}
