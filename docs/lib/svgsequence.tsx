import { useEffect, useState } from 'react';

export function SVGSequence({ imgSize = '100%', svgUrls, interval = 2000 }) {
    const [current, setCurrent] = useState(0);

    useEffect(() => {
        const timer = setInterval(() => {
            setCurrent(prev => (prev + 1) % svgUrls.length);
        }, interval);

        return () => clearInterval(timer);
    }, [svgUrls, interval]);

    return (
        <div className="svg-animation-container" style={{ position: 'relative', width: '100%', backgroundColor: 'transparent' }}>
            {svgUrls.map((url, index) => (
                <img
                    key={url}
                    src={url}
                    alt={`SVG frame ${index}`}
                    style={{
                        position: index === current ? 'relative' : 'absolute',
                        top: 0,
                        left: 0,
                        width: imgSize,
                        height: 'auto',
                        margin: 'auto',
                        opacity: index === current ? 1 : 0,
                        pointerEvents: index === current ? 'auto' : 'none',
                        transition: 'linear',
                        backgroundColor: 'transparent',
                    }}
                />
            ))}
        </div>
    );
}
