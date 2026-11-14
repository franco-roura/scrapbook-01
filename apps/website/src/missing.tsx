// Entry for the temporary, dev-only /missing.html. See MissingPage.tsx.
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import "./index.css";
import { MissingPage } from "./MissingPage";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <MissingPage />
  </StrictMode>,
);
