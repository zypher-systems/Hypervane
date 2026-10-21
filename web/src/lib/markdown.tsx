import type { Components } from "react-markdown";
import Markdown from "react-markdown";
import remarkBreaks from "remark-breaks";
import remarkGfm from "remark-gfm";

function safeUrl(url: string) {
  return /^https?:\/\//i.test(url) ? url : "";
}

const components: Components = {
  a: ({ href, children }) => (
    <a href={href} target="_blank" rel="noreferrer" className="text-accent underline-offset-2 hover:underline">
      {children}
    </a>
  ),
  img: () => null,
  h1: ({ children }) => <h1 className="text-base font-semibold text-ink">{children}</h1>,
  h2: ({ children }) => <h2 className="text-sm font-semibold text-ink">{children}</h2>,
  h3: ({ children }) => <h3 className="text-sm font-medium text-ink">{children}</h3>,
  p: ({ children }) => <p className="text-sm leading-relaxed text-ink">{children}</p>,
  ul: ({ children }) => <ul className="list-disc space-y-1 pl-4 text-sm">{children}</ul>,
  ol: ({ children }) => <ol className="list-decimal space-y-1 pl-4 text-sm">{children}</ol>,
  li: ({ children }) => <li className="text-sm text-ink">{children}</li>,
  blockquote: ({ children }) => (
    <blockquote className="border-l-2 border-accent/50 pl-3 text-sm text-ink-dim">{children}</blockquote>
  ),
  code: ({ children }) => (
    <code className="rounded bg-white/10 px-1 py-0.5 font-mono text-[11px]">{children}</code>
  ),
  pre: ({ children }) => (
    <pre className="overflow-x-auto rounded-md bg-black/40 px-3 py-2 font-mono text-[11px] text-ink-dim">{children}</pre>
  ),
  table: ({ children }) => (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-left text-xs">{children}</table>
    </div>
  ),
  th: ({ children }) => <th className="border-b border-white/10 py-1 pr-3 font-medium text-ink">{children}</th>,
  td: ({ children }) => <td className="border-b border-white/[0.06] py-1 pr-3 text-ink-dim">{children}</td>,
  hr: () => <hr className="border-white/10" />,
};

export function MarkdownView({ text }: { text: string }) {
  const src = text.replace(/\r\n/g, "\n").trim();
  if (!src) return <p className="text-sm text-ink-faint">No notes</p>;
  return (
    <div className="hv-md space-y-2">
      <Markdown remarkPlugins={[remarkGfm, remarkBreaks]} urlTransform={safeUrl} components={components}>
        {src}
      </Markdown>
    </div>
  );
}
