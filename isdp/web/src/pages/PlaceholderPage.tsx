import React from 'react';
import { Result } from 'antd';

interface PlaceholderPageProps {
  title: string;
  description?: string;
}

const PlaceholderPage: React.FC<PlaceholderPageProps> = ({ title, description }) => {
  return (
    <Result
      status="info"
      title={title}
      subTitle={description || '该功能正在开发中，敬请期待'}
    />
  );
};

export default PlaceholderPage;