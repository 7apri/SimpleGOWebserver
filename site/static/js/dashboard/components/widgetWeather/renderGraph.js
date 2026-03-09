import { stupne } from "../../../util/misc.js";

/*
 * Renders the day's graf into the DOM element provided.
 * By default its the el with id=graf
 *
 * @param {Array<number>} temps - The temperatures to render.
 * @param {HTMLElement} element - The graph's parent wrapper.
 * @returns {boolean} Success status.
 */
const renderGraph = (temps, element = document.getElementById("graf")) => {
    try {
        if (element._cleanupGraph) {
            element._cleanupGraph();
        }

        temps = temps.map(val => Math.round(val));
        
        if (temps.length === 0) return false;

        let minTemp = Math.min(...temps);
        let maxTemp = Math.max(...temps);

        if (minTemp === maxTemp) {
            minTemp -= 1;
            maxTemp += 1;
        }
        

        const grafTemps = Array.from(element.querySelector("#graf-temps").children).entries();
        for (const [index, childElement] of grafTemps) {
            const textElement = childElement.querySelector(".weather-day-graf-temp");
            const srElement = childElement.querySelector(".sr-only");
            if (!textElement || !srElement) continue;
            if (index >= temps.length) break;
            
            textElement.classList.remove("cold");
            if(temps[index] < 0) textElement.classList.add("cold");
            
            textElement.textContent = Math.abs(temps[index]);
            
            srElement.textContent = `${["Ráno", "Ve dne", "Večer", "V noci"][index]} je ${temps[index]} ${stupne(temps[index])}`;
        }

        const canvas = element.querySelector(".weather-day-graf");
        const ctx = canvas.getContext('2d');
        
        const renderCanvas = () => {
            const displayWidth = canvas.clientWidth;
            const displayHeight = canvas.clientHeight;
            
            if (displayWidth === 0 || displayHeight === 0) return;

            const dpr = window.devicePixelRatio || 1;
            canvas.width = displayWidth * dpr;
            canvas.height = displayHeight * dpr;

            ctx.setTransform(1, 0, 0, 1, 0, 0); 
            ctx.scale(dpr, dpr);

            const paddingSide = 15; 
            const paddingTopBottom = 15;
            
            const drawingHeight = displayHeight - (paddingTopBottom * 2);
            const range = maxTemp - minTemp;

            const stepX = (displayWidth - paddingSide * 2) / (temps.length - 1);

            ctx.clearRect(0, 0, displayWidth, displayHeight);
            
            const rootStyles = getComputedStyle(document.documentElement);
            const fgColor = rootStyles.getPropertyValue('--fg-color-high').trim();

            ctx.strokeStyle = fgColor;
            ctx.lineWidth = Math.round(Math.max(5, Math.min(displayWidth, displayHeight) / 70) );
            ctx.lineJoin = 'round';
            ctx.lineCap = 'round';
            ctx.beginPath();

            temps.forEach((val, i) => {
                const x = paddingSide + i * stepX;
                const normalizedY = (val - minTemp) / range;
                const y = displayHeight - paddingTopBottom - (normalizedY * drawingHeight);
                
                if (i === 0) ctx.moveTo(x, y);
                else ctx.lineTo(x, y);
            });
            ctx.stroke();

            ctx.fillStyle = fgColor;
            temps.forEach((val, i) => {
                const x = paddingSide + i * stepX;
                const normalizedY = (val - minTemp) / range;
                const y = displayHeight - paddingTopBottom - (normalizedY * drawingHeight);
                
                ctx.beginPath();
                ctx.arc(x, y, ctx.lineWidth, 0, Math.PI * 2);
                ctx.fill();
            });
        };

        window.addEventListener('resize', renderCanvas);
        document.addEventListener('DOMContentLoaded', renderCanvas);

        const observer = new MutationObserver((mutations) => {
            mutations.forEach((m) => {
                if (m.attributeName === 'data-theme') renderCanvas();
            });
        });
        observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });

        renderCanvas();

        element._cleanupGraph = () => {
            window.removeEventListener('resize', renderCanvas);
            document.removeEventListener('DOMContentLoaded', renderCanvas);
            observer.disconnect();
        };

        return true;
    } catch (err) {
        console.error("Graph error:", err);
        return false;
    }
};
export default renderGraph;
