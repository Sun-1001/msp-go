import { MainLayout } from '@/components/layout/MainLayout';
import { NoteWorkspace } from '@/modules/notes';
import { selectCurrentUser } from '@/modules/auth/state';
import { useAppSelector } from '@/store';

export function NotesPage() {
  const user = useAppSelector(selectCurrentUser);
  return <MainLayout><NoteWorkspace key={user?.id} /></MainLayout>;
}
