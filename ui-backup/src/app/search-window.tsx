import {FixedSizeList} from 'react-window';

const SearchWindow = (props) => {
    const {options, children, getValue} = props;
    const [value] = getValue();
    const initialOffset = options.indexOf(value) * 35;

    return (
        <FixedSizeList
            height={300}
            itemCount={children.length}
            itemSize={35}
            initialScrollOffset={initialOffset}
        >
            {({index, style}) => <div style={style}>{children[index]}</div>}
        </FixedSizeList>
    );
};

export default SearchWindow;