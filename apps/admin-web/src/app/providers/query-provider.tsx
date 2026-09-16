import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { PropsWithChildren } from "react";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (attempt, error) => {
        const status = (error as { status?: number } | null)?.status;
        return (status === undefined || status >= 500) && attempt < 2;
      },
      refetchOnWindowFocus: false,
    },
    mutations: { retry: false },
  },
});

export function QueryProvider({ children }: PropsWithChildren) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}
