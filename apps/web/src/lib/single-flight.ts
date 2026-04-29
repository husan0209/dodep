export interface SingleFlight<T> {
  run: (task: () => Promise<T>) => Promise<T>;
  isRunning: () => boolean;
}

export function createSingleFlight<T>(): SingleFlight<T> {
  let inFlight: Promise<T> | null = null;

  return {
    run(task: () => Promise<T>): Promise<T> {
      if (inFlight) {
        return inFlight;
      }

      inFlight = (async () => task())();
      return inFlight.finally(() => {
        inFlight = null;
      });
    },
    isRunning(): boolean {
      return inFlight !== null;
    },
  };
}
