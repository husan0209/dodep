import { create } from "zustand";

export interface Selection {
  eventId: number;
  marketId: number;
  outcomeId: number;
  outcomeName: string;
  odds: number;
  eventName: string;
  marketName: string;
}

interface BetSlipState {
  // State
  selections: Selection[];
  stake: number;
  betType: "single" | "accumulator" | "system";

  // Computed
  combinedOdds: () => number;
  potentialWin: () => number;

  // Actions
  addSelection: (selection: Selection) => void;
  removeSelection: (outcomeId: number) => void;
  toggleSelection: (selection: Selection) => void;
  updateOdds: (outcomeId: number, newOdds: number) => void;
  setStake: (stake: number) => void;
  setBetType: (type: "single" | "accumulator" | "system") => void;
  clear: () => void;
}

export const useBetSlipStore = create<BetSlipState>((set, get) => ({
  selections: [],
  stake: 0,
  betType: "single",

  combinedOdds: () => {
    const { selections, betType } = get();
    if (betType === "single" || selections.length <= 1) {
      return selections[0]?.odds ?? 0;
    }
    return selections.reduce((acc, s) => acc * s.odds, 1);
  },

  potentialWin: () => {
    const stake = get().stake;
    return stake * get().combinedOdds();
  },

  addSelection: (selection) => {
    set((state) => {
      // Max 20 selections
      if (state.selections.length >= 20) return state;

      // Remove other outcomes from same market in the same event.
      const filtered = state.selections.filter(
        (s) => !(s.eventId === selection.eventId && s.marketId === selection.marketId)
      );

      return { selections: [...filtered, selection] };
    });
  },

  removeSelection: (outcomeId) => {
    set((state) => ({
      selections: state.selections.filter((s) => s.outcomeId !== outcomeId),
    }));
  },

  toggleSelection: (selection) => {
    const exists = get().selections.find(
      (s) => s.outcomeId === selection.outcomeId
    );
    if (exists) {
      get().removeSelection(selection.outcomeId);
    } else {
      get().addSelection(selection);
    }
  },

  updateOdds: (outcomeId, newOdds) => {
    set((state) => ({
      selections: state.selections.map((s) =>
        s.outcomeId === outcomeId ? { ...s, odds: newOdds } : s
      ),
    }));
  },

  setStake: (stake) => set({ stake: Math.max(0, stake) }),
  setBetType: (betType) => set({ betType }),
  clear: () => set({ selections: [], stake: 0, betType: "single" }),
}));
