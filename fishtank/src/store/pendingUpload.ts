/** In-memory handoff from Home → Process (/process/new) before ontology API runs */

export type PendingUploadState = {
  files: File[]
  simulationRequirement: string
  isPending: boolean
}

const state: PendingUploadState = {
  files: [],
  simulationRequirement: '',
  isPending: false
}

export function setPendingUpload(files: File[], requirement: string) {
  state.files = files
  state.simulationRequirement = requirement
  state.isPending = true
}

export function getPendingUpload(): PendingUploadState {
  return {
    files: state.files,
    simulationRequirement: state.simulationRequirement,
    isPending: state.isPending
  }
}

export function clearPendingUpload() {
  state.files = []
  state.simulationRequirement = ''
  state.isPending = false
}
