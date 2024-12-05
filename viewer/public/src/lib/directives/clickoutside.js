export function clickOutside(node) {
    const handleClick = e => {
        if (node && !node.contains(e.target) && !e.defaultPrevented) {
            node.dispatchEvent(new CustomEvent('clickOutside', node));
        }
    }

    document.addEventListener('click', handleClick, {capture: true});

    return {
        destroy() {
            document.removeEventListener('click', handleClick, {capture: true});
        }
    }
}