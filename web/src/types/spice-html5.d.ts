declare module "@spice-project/spice-html5" {
  export class SpiceMainConn {
    constructor(opts: {
      uri: string;
      screen_id?: string;
      password?: string;
      onerror?: (e: unknown) => void;
      onsuccess?: () => void;
    });
    stop(): void;
  }
}
