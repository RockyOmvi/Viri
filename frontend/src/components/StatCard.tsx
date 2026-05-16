interface StatCardProps {
  label: string
  value: string
  subtext?: string
  accentColor?: string
  icon?: string
  live?: boolean
}

export default function StatCard({ label, value, subtext, accentColor = 'border-l-primary', icon, live }: StatCardProps) {
  return (
    <div className={`bg-surface-container border border-outline-variant rounded-lg p-6 relative overflow-hidden group hover:bg-surface-container-high transition-colors border-l-4 ${accentColor}`}>
      {icon && (
        <div className="absolute top-0 right-0 p-4 opacity-5 text-primary pointer-events-none">
          <span className="text-6xl material-symbols-outlined" style={{ fontVariationSettings: "'FILL' 1" }}>{icon}</span>
        </div>
      )}
      <div className="flex justify-between items-start mb-2">
        <h3 className="font-label-caps text-label-caps text-on-surface-variant">{label}</h3>
        {live && (
          <div className="flex items-center gap-1 bg-tertiary-container text-on-tertiary-fixed px-2 py-0.5 rounded-full">
            <span className="w-2 h-2 bg-tertiary rounded-full animate-pulse"></span>
            <span className="font-mono-sm text-mono-sm">Live</span>
          </div>
        )}
      </div>
      <div className="font-headline-lg text-headline-lg font-bold text-on-surface font-mono-base">
        {value}
      </div>
      {subtext && (
        <div className="text-body-sm font-body-sm text-on-surface-variant mt-2 flex items-center gap-1">
          {subtext}
        </div>
      )}
    </div>
  )
}
