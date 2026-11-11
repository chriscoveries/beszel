import { Activity } from "lucide-react"
import { cn } from "@/lib/utils"
import type { CifastInfo } from "@/types"

function formatCount(value?: number) {
	return value === undefined ? "—" : Math.round(value).toLocaleString()
}

function formatDuration(value?: number) {
	if (value === undefined) return "—"
	const seconds = Math.max(0, Math.round(value))
	if (seconds < 60) return `${seconds}s`
	if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
	if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
	return `${Math.floor(seconds / 86400)}d ${Math.floor((seconds % 86400) / 3600)}h`
}

function formatWindow(seconds: number) {
	if (seconds % 86400 === 0) return `${seconds / 86400}d`
	if (seconds % 3600 === 0) return `${seconds / 3600}h`
	return formatDuration(seconds)
}

function formatAge(capturedAt: number) {
	const seconds = Math.max(0, Math.floor(Date.now() / 1000) - capturedAt)
	return seconds < 5 ? "just now" : `${formatDuration(seconds)} ago`
}

function Metric({ label, value }: { label: string; value: string }) {
	return (
		<div className="min-w-0 rounded-lg border bg-muted/20 px-3 py-3 sm:px-4">
			<div className="text-xs text-muted-foreground truncate">{label}</div>
			<div className="mt-1 text-xl sm:text-2xl font-semibold tabular-nums truncate">{value}</div>
		</div>
	)
}

export default function CifastCard({ data }: { data?: CifastInfo }) {
	if (!data) return null

	const unavailable = data.s === "STALE" || data.s === "UNOBSERVED"
	const statusClass = cn("rounded-full px-2.5 py-1 text-xs font-medium", {
		"bg-green-500/15 text-green-700 dark:text-green-400": data.s === "OK",
		"bg-yellow-500/15 text-yellow-700 dark:text-yellow-400": data.s === "PARTIAL",
		"bg-red-500/15 text-red-700 dark:text-red-400": unavailable,
	})
	const window = formatWindow(data.w)

	return (
		<div className="border-t px-4 py-5 sm:px-6 sm:py-6">
			<div className="flex flex-wrap items-start justify-between gap-3">
				<div>
					<div className="flex items-center gap-2">
						<Activity className="size-5" />
						<h2 className="text-lg font-semibold">CIFast</h2>
					</div>
					<p className="mt-1 text-sm text-muted-foreground">
						Open {data.oc.toLowerCase()}, history {data.hc.toLowerCase()} · updated {formatAge(data.at)}
					</p>
				</div>
				<span className={statusClass}>{data.s}</span>
			</div>

			<div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-6">
				<Metric label="Running" value={unavailable ? "—" : formatCount(data.r)} />
				<Metric label="Queued" value={unavailable ? "—" : formatCount(data.q)} />
				<Metric label="Oldest queue" value={unavailable ? "—" : formatDuration(data.oq)} />
				<Metric label={`Finished · ${window}`} value={unavailable ? "—" : formatCount(data.f)} />
				<Metric label="Queue p90" value={unavailable ? "—" : formatDuration(data.q90)} />
				<Metric label="Run p90" value={unavailable ? "—" : formatDuration(data.r90)} />
			</div>
			{!unavailable && data.x !== undefined && data.x > 0 && (
				<p className="mt-3 text-sm text-yellow-700 dark:text-yellow-400">
					{formatCount(data.x)} job{data.x === 1 ? "" : "s"} vanished during the {window} window.
				</p>
			)}
		</div>
	)
}
