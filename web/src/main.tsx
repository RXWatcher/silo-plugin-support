import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "@/App";
import { captureTokenFromURL } from "@/lib/authToken";
import "./index.css";

captureTokenFromURL();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
