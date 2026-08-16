/**
 * Usage page display preferences (persisted to localStorage).
 * Controls the optional per-request metrics shown in the Request Log.
 */

import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { STORAGE_KEY_USAGE_PREFS } from '@/utils/constants';

interface UsagePrefsState {
  /** Show the estimated cost column in the Request Log. */
  showLogCost: boolean;
  /** Show the input/output/reasoning token pie in the Request Log. */
  showLogTokenPie: boolean;
  setShowLogCost: (value: boolean) => void;
  setShowLogTokenPie: (value: boolean) => void;
}

export const useUsagePrefsStore = create<UsagePrefsState>()(
  persist(
    (set) => ({
      showLogCost: true,
      showLogTokenPie: true,
      setShowLogCost: (value) => set({ showLogCost: value }),
      setShowLogTokenPie: (value) => set({ showLogTokenPie: value }),
    }),
    {
      name: STORAGE_KEY_USAGE_PREFS,
    }
  )
);
