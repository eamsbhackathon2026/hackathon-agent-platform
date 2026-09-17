import { createContext, useContext, useLayoutEffect } from "react";

/**
 * The app shell titles every page from the navigation entry it matches. Detail pages carry a title
 * the navigation cannot know (an assistant name, a conversation title), so they publish one here.
 * The default is a no-op so a page still renders when it is mounted outside the shell.
 */
export type PageHeaderContent = { title: string; description?: string | undefined };

export type SetPageHeader = (content: PageHeaderContent | null) => void;

export const PageHeaderContext = createContext<SetPageHeader>(() => {});

/** Replaces the shell header title while the calling page is mounted. Pass `null` while data loads. */
export function usePageHeader(content: PageHeaderContent | null) {
  const setPageHeader = useContext(PageHeaderContext);
  const title = content?.title;
  const description = content?.description;
  // Layout effect so the header never paints the previous route's title for a frame.
  useLayoutEffect(() => {
    setPageHeader(title ? { title, description } : null);
    return () => setPageHeader(null);
  }, [description, setPageHeader, title]);
}
