import { Modal } from '@/components/ui/Modal';

interface MessageImagePreviewProps {
  image: {
    url: string;
    name: string;
  } | null;
  onClose: () => void;
}

export function MessageImagePreview({ image, onClose }: MessageImagePreviewProps) {
  if (!image) return null;

  return (
    <Modal
      isOpen
      onClose={onClose}
      showHeader={false}
      ariaLabel={`查看图片 ${image.name}`}
      className="max-w-5xl p-4"
    >
      <img
        src={image.url}
        alt={image.name}
        className="relative z-10 max-h-[calc(100vh-4rem)] w-full rounded-md object-contain"
      />
    </Modal>
  );
}
