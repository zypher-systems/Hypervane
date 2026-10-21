/// <reference types="vite/client" />

declare module "@novnc/novnc" {
  export default class RFB {
    constructor(target: HTMLElement, url: string, options?: Record<string, unknown>);
    scaleViewport: boolean;
    resizeSession: boolean;
    showDotCursor: boolean;
    background: string;
    disconnect(): void;
    addEventListener?(name: string, handler: (ev: { detail?: { clean?: boolean } }) => void): void;
  }
}
