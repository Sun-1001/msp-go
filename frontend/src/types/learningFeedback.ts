/** Recent assessment evidence, independent of the selected statistics range. */
export interface LearningFeedback {
  /** Missing on snapshots generated before assistance evidence was introduced. */
  evidence_version?: number;
  concept_id: string;
  concept_name: string;
  stage: 'diagnostic' | 'repair' | 'consolidate' | 'verify' | 'retention' | 'transfer';
  label: string;
  reason: string;
  next_step: string;
  success_criterion: string;
  recommended_difficulty: number;
  estimated_mastery: number | null;
  confidence: number;
  recent_attempts: number;
  independent_attempts: number;
  independent_correct: number;
  hinted_attempts?: number;
  hinted_correct?: number;
  unknown_assistance_attempts?: number;
  delayed_correct: number;
  recent_accuracy: number | null;
  dominant_error?: string;
  focus_error?: string;
  window_days: number;
  as_of: string;
}
