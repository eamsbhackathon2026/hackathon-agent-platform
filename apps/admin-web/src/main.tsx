import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { Toaster, TooltipProvider } from "@/shared/ui";

import { AppRouterProvider } from "./app/providers/router-provider";
import { QueryProvider } from "./app/providers/query-provider";
import "./app/styles/index.css";

const root = document.getElementById("root");
if (!root) throw new Error("Application root element was not found.");

createRoot(root).render(
  <StrictMode>
    <QueryProvider>
      <TooltipProvider><AppRouterProvider /><Toaster richColors /></TooltipProvider>
    </QueryProvider>
  </StrictMode>,
);
