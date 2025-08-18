import { Box, Skeleton } from '@mui/joy'

const SkeletonLoader = ({
  type = 'card',
  count = 1,
  height = 100,
  width = '100%',
  variant = 'rectangular',
  ...props
}) => {
  const renderSkeleton = () => {
    switch (type) {
      case 'card':
        return (
          <Box
            sx={{
              p: 2,
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 'md',
            }}
          >
            <Skeleton variant='text' height={24} width='60%' sx={{ mb: 1 }} />
            <Skeleton
              variant='text'
              height={16}
              width='100%'
              sx={{ mb: 0.5 }}
            />
            <Skeleton variant='text' height={16} width='80%' sx={{ mb: 2 }} />
            <Box sx={{ display: 'flex', gap: 1 }}>
              <Skeleton variant='circular' width={32} height={32} />
              <Skeleton variant='rectangular' height={32} width={80} />
            </Box>
          </Box>
        )

      case 'list':
        return (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, p: 1.5 }}>
            <Skeleton variant='circular' width={40} height={40} />
            <Box sx={{ flex: 1 }}>
              <Skeleton
                variant='text'
                height={20}
                width='70%'
                sx={{ mb: 0.5 }}
              />
              <Skeleton variant='text' height={16} width='50%' />
            </Box>
            <Skeleton variant='rectangular' width={60} height={24} />
          </Box>
        )

      case 'chore':
        return (
          <Box
            sx={{
              p: 2,
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 'md',
            }}
          >
            <Box
              sx={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'flex-start',
                mb: 1,
              }}
            >
              <Skeleton variant='text' height={24} width='50%' />
              <Skeleton variant='circular' width={24} height={24} />
            </Box>
            <Skeleton variant='text' height={16} width='80%' sx={{ mb: 1 }} />
            <Box sx={{ display: 'flex', gap: 1, mb: 2 }}>
              <Skeleton variant='rectangular' height={20} width={60} />
              <Skeleton variant='rectangular' height={20} width={40} />
            </Box>
            <Box
              sx={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
              }}
            >
              <Skeleton variant='circular' width={32} height={32} />
              <Skeleton variant='text' height={16} width='30%' />
            </Box>
          </Box>
        )

      case 'profile':
        return (
          <Box
            sx={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              p: 3,
            }}
          >
            <Skeleton
              variant='circular'
              width={80}
              height={80}
              sx={{ mb: 2 }}
            />
            <Skeleton variant='text' height={24} width={150} sx={{ mb: 1 }} />
            <Skeleton variant='text' height={16} width={100} />
          </Box>
        )

      default:
        return (
          <Skeleton
            variant={variant}
            height={height}
            width={width}
            {...props}
          />
        )
    }
  }

  return (
    <>
      {Array.from({ length: count }, (_, index) => (
        <Box key={index} sx={{ mb: type === 'list' ? 0 : 2 }}>
          {renderSkeleton()}
        </Box>
      ))}
    </>
  )
}

export default SkeletonLoader
