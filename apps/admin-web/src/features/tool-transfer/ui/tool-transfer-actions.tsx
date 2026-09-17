import { Download, Upload } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/shared/ui";
import { exportTools } from "../api/tool-transfer";
import { downloadBundle, transferErrorMessage } from "../lib/bundle-file";
import { ToolImportDialog } from "./tool-import-dialog";

/** Export is open to everyone who can see tools; import only to people who can edit them. */
export function ToolTransferActions({ canImport, onImported }: { canImport: boolean; onImported: () => Promise<unknown> }) {
  const [importing, setImporting] = useState(false);
  const [exporting, setExporting] = useState(false);
  const download = async () => {
    setExporting(true);
    try {
      downloadBundle(await exportTools());
    } catch (error) {
      toast.error("Unable to export tools", { description: transferErrorMessage(error, "Try again in a moment."), action: { label: "Try again", onClick: () => void download() } });
    } finally { setExporting(false); }
  };
  return <>
    <Button variant="outline" disabled={exporting} onClick={() => void download()}><Download />{exporting ? "Exporting…" : "Export tools"}</Button>
    {canImport ? <><Button variant="outline" onClick={() => setImporting(true)}><Upload />Import tools</Button><ToolImportDialog open={importing} onOpenChange={setImporting} onImported={onImported} /></> : null}
  </>;
}
