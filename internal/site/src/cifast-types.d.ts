import "@/types"

declare module "@/types" {
	export interface CifastInfo {
		/** snapshot capture time, Unix seconds */
		at: number
		/** CIFast snapshot status */
		s: "OK" | "PARTIAL" | "UNOBSERVED" | "STALE"
		/** current/open-job coverage */
		oc: "OBSERVED" | "PARTIAL" | "STALE" | "UNOBSERVED"
		/** history coverage */
		hc: "OBSERVED" | "PARTIAL" | "STALE" | "UNOBSERVED"
		/** aggregation window, seconds */
		w: number
		/** queued jobs */
		q?: number
		/** running jobs */
		r?: number
		/** oldest queue age, seconds */
		oq?: number
		/** jobs finished in the window */
		f?: number
		/** jobs that vanished in the window */
		x?: number
		/** queue p50, seconds */
		q50?: number
		/** queue p90, seconds */
		q90?: number
		/** run p50, seconds */
		r50?: number
		/** run p90, seconds */
		r90?: number
	}

	export interface SystemInfo {
		/** current aggregate CIFast snapshot */
		ci?: CifastInfo
	}
}
