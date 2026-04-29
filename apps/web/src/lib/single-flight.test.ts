import { createSingleFlight } from "./single-flight";

describe("createSingleFlight", () => {
  test("deduplicates concurrent register attempts", async () => {
    const gate = createSingleFlight<string>();
    const task = jest.fn(
      async () => new Promise<string>((resolve) => setTimeout(() => resolve("ok"), 10))
    );

    const [first, second] = await Promise.all([gate.run(task), gate.run(task)]);

    expect(first).toBe("ok");
    expect(second).toBe("ok");
    expect(task).toHaveBeenCalledTimes(1);
  });

  test("allows next registration after previous one finishes", async () => {
    const gate = createSingleFlight<string>();
    const task = jest
      .fn<Promise<string>, []>()
      .mockResolvedValueOnce("user-1")
      .mockResolvedValueOnce("user-2");

    const first = await gate.run(task);
    const second = await gate.run(task);

    expect(first).toBe("user-1");
    expect(second).toBe("user-2");
    expect(task).toHaveBeenCalledTimes(2);
    expect(gate.isRunning()).toBe(false);
  });
});
