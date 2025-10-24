import SwaggerUI from 'swagger-ui-react';
import 'swagger-ui-react/swagger-ui.css';

const SwaggerViewer = ({ url }: { url: string }) => {
    return <SwaggerUI url={url} />;
};

export default SwaggerViewer;
