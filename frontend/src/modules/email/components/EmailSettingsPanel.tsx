import React from 'react';
import { EmailTemplatesCard } from '@/modules/email/components/EmailTemplatesCard';
import { SMTPSettingsCard } from '@/modules/email/components/SMTPSettingsCard';

export const EmailSettingsPanel: React.FC = () => (
  <div className="space-y-6">
    <SMTPSettingsCard />
    <EmailTemplatesCard />
  </div>
);
