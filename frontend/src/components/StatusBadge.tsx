interface StatusBadgeProps {
  status: 'success' | 'pending' | 'failed'
}

export default function StatusBadge({ status }: StatusBadgeProps) {
  const styles = {
    success: 'bg-tertiary-container/20 text-tertiary border-tertiary/30',
    pending: 'bg-primary-container/20 text-primary border-primary/30',
    failed: 'bg-error-container/20 text-error border-error/30',
  }

  return (
    <span className={`${styles[status]} border px-2 py-0.5 rounded font-label-caps text-label-caps`}>
      {status === 'success' ? 'Success' : status === 'pending' ? 'Pending' : 'Failed'}
    </span>
  )
}
