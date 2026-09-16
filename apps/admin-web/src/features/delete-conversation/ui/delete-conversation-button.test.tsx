import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { http } from "msw";
import { describe, expect, it, vi } from "vitest";

import { server } from "@/test/msw-server";
import { conversationKeys } from "@/entities/conversation";
import { DeleteConversationButton } from "./delete-conversation-button";

describe("DeleteConversationButton", () => {
  it("requires confirmation before deleting", async () => {
    const deleted = vi.fn();
    const onDeleted = vi.fn();
    server.use(http.delete("*/v1/sessions/:sessionId", () => { deleted(); return new Response(null, { status: 204 }); }));
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    const invalidate = vi.spyOn(client, "invalidateQueries");
    render(<QueryClientProvider client={client}><DeleteConversationButton conversationId="session-1" ariaLabel="Delete First chat" compact onDeleted={onDeleted} /></QueryClientProvider>);
    const trigger = screen.getByRole("button", { name: "Delete First chat" });
    expect(trigger).toHaveClass("size-11");
    fireEvent.click(trigger);
    expect(await screen.findByRole("heading", { name: "Delete this conversation?" })).toBeInTheDocument();
    expect(deleted).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Delete conversation" }));
    await waitFor(() => expect(deleted).toHaveBeenCalledTimes(1));
    expect(invalidate).toHaveBeenCalledWith({ queryKey: conversationKeys.all });
    expect(onDeleted).toHaveBeenCalledTimes(1);
  });

  it("does not open when disabled", () => {
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    render(<QueryClientProvider client={client}><DeleteConversationButton conversationId="session-1" disabled /></QueryClientProvider>);

    const trigger = screen.getByRole("button", { name: "Delete conversation" });
    expect(trigger).toBeDisabled();
    fireEvent.click(trigger);
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });
});
