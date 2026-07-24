declare module 'three-spritetext' {
  import { Sprite, SpriteMaterial, Vector3 } from 'three';

  export default class SpriteText extends Sprite {
    constructor(text?: string, textHeight?: number, color?: string);
    text: string;
    textHeight: number;
    color: string;
    backgroundColor: string | false;
    padding: number | number[];
    borderWidth: number;
    borderColor: string;
    borderRadius: number;
    fontFace: string;
    fontWeight: string;
    center: { y: number };
    position: Vector3;
    renderOrder: number;
    material: SpriteMaterial;
  }
}
