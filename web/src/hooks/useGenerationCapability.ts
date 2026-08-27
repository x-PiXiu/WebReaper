import { useGenerationTypes } from './useGenerationTypes'

/** @deprecated 请用 useGenerationTypes */
export function useGenerationCapability(subType: string) {
  const { isEnabled, isLoading, isError } = useGenerationTypes()
  return {
    isLoading,
    isError,
    enabled: isEnabled(subType),
    missing: !isLoading && !isError && !isEnabled(subType),
  }
}
